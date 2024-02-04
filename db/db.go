package db

import (
	"context"
	"errors"
	"fmt"
	"github.com/gobicycle/bicycle/audit"
	"github.com/gobicycle/bicycle/config"
	"github.com/gobicycle/bicycle/core"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	log "github.com/sirupsen/logrus"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"sync"
	"time"
)

type Connection struct {
	client      *pgxpool.Pool
	addressBook addressBook
}

type addressBook struct {
	addresses map[core.Address]core.AddressInfo
	mutex     sync.Mutex
}

func (ab *addressBook) get(address core.Address) (core.AddressInfo, bool) {
	ab.mutex.Lock()
	t, ok := ab.addresses[address]
	ab.mutex.Unlock()
	return t, ok
}

func (ab *addressBook) put(address core.Address, t core.AddressInfo) {
	ab.mutex.Lock()
	ab.addresses[address] = t
	ab.mutex.Unlock()
}

func NewConnection(URI string) (*Connection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	client, err := pgxpool.Connect(ctx, URI)
	if err != nil {
		return nil, fmt.Errorf("connection err: %v", err)
	}
	conn := Connection{client, addressBook{}}
	return &conn, nil
}

func (c *Connection) GetWalletType(address core.Address) (core.WalletType, bool) {
	info, ok := c.addressBook.get(address)
	return info.Type, ok
}

func (c *Connection) GetUserID(address core.Address) (string, bool) {
	info, ok := c.addressBook.get(address)
	return info.UserID, ok
}

// GetOwner returns owner for jetton deposit from address book and nil for other types
func (c *Connection) GetOwner(address core.Address) *core.Address {
	info, ok := c.addressBook.get(address)
	if ok && info.Type == core.JettonDepositWallet && info.Owner == nil {
		log.Fatalf("must be owner address in address book for jetton deposit")
	}
	return info.Owner
}

func (c *Connection) GetWalletTypeByTonutilsAddress(address *address.Address) (core.WalletType, bool) {
	a, err := core.AddressFromTonutilsAddress(address)
	if err != nil {
		return "", false
	}
	return c.GetWalletType(a)
}

// GetLastSubwalletID returns last (greatest) used subwallet_id from DB
// numeration starts from wallet.DefaultSubwallet (this number reserved for main hot wallet)
// returns wallet.DefaultSubwallet if table is empty
func (c *Connection) GetLastSubwalletID(ctx context.Context) (uint32, error) {
	var id uint32
	err := c.client.QueryRow(ctx, `
		SELECT COALESCE(MAX(subwallet_id), $1)
		FROM payments.ton_wallets
	`, wallet.DefaultSubwallet).Scan(&id)
	return id, err
}

func (c *Connection) SaveTonWallet(ctx context.Context, walletData core.WalletData) error {
	_, err := c.client.Exec(ctx, `
		INSERT INTO payments.ton_wallets (
		user_id,
		subwallet_id,
		type,
		address)
		VALUES ($1, $2, $3,$4)         
	`,
		walletData.UserID,
		walletData.SubwalletID,
		walletData.Type,
		walletData.Address,
	)
	if err != nil {
		return err
	}
	c.addressBook.put(walletData.Address, core.AddressInfo{Type: walletData.Type, Owner: nil, UserID: walletData.UserID})
	return nil
}

func (c *Connection) GetJettonWallet(ctx context.Context, address core.Address) (*core.WalletData, bool, error) {
	d := core.WalletData{
		Address: address,
	}
	err := c.client.QueryRow(ctx, `
		SELECT subwallet_id, user_id, currency, type
		FROM payments.jetton_wallets
		WHERE  address = $1
	`, address).Scan(&d.SubwalletID, &d.UserID, &d.Currency, &d.Type)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &d, true, nil
}

func (c *Connection) SaveJettonWallet(
	ctx context.Context,
	ownerAddress core.Address,
	walletData core.WalletData,
	notSaveOwner bool,
) error {
	tx, err := c.client.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if !notSaveOwner {
		_, err = tx.Exec(ctx, `
		INSERT INTO payments.ton_wallets (
		user_id,
		subwallet_id,
		type,
		address)
		VALUES ($1, $2, $3,$4)            
	`,
			walletData.UserID,
			walletData.SubwalletID,
			core.JettonOwner,
			ownerAddress,
		)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO payments.jetton_wallets (
		user_id,
		subwallet_id,
		currency,
		type,
		address)
		VALUES ($1, $2, $3, $4, $5)
	`,
		walletData.UserID,
		walletData.SubwalletID,
		walletData.Currency,
		walletData.Type,
		walletData.Address,
	)
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	if walletData.Type == core.JettonDepositWallet {
		// only jetton deposit owners tracked by address book
		// hot TON wallet also owner of jetton hot wallets
		// cold wallets excluded from address book
		c.addressBook.put(ownerAddress, core.AddressInfo{Type: core.JettonOwner, Owner: nil})
	}
	c.addressBook.put(walletData.Address, core.AddressInfo{Type: walletData.Type, Owner: &ownerAddress, UserID: walletData.UserID})
	return nil
}

func (c *Connection) GetTonWalletsAddresses(
	ctx context.Context,
	userID string,
	types []core.WalletType,
) (
	[]core.Address,
	error,
) {
	if types == nil {
		types = make([]core.WalletType, 0)
	}
	rows, err := c.client.Query(ctx, `
		SELECT address
		FROM payments.ton_wallets
		WHERE user_id = $1 AND type=ANY($2)
	`, userID, types)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []core.Address
	for rows.Next() {
		var a core.Address
		err = rows.Scan(&a)
		if err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, nil
}

func (c *Connection) GetJettonOwnersAddresses(
	ctx context.Context,
	userID string,
	types []core.WalletType,
) (
	[]core.OwnerWallet,
	error,
) {
	if types == nil {
		types = make([]core.WalletType, 0)
	}
	rows, err := c.client.Query(ctx, `
		SELECT tw.address, jw.currency
		FROM payments.jetton_wallets jw
		LEFT JOIN payments.ton_wallets tw ON jw.subwallet_id = tw.subwallet_id 
		WHERE jw.user_id = $1 AND jw.type=ANY($2)
	`, userID, types)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []core.OwnerWallet
	for rows.Next() {
		var ow core.OwnerWallet
		err = rows.Scan(&ow.Address, &ow.Currency)
		if err != nil {
			return nil, err
		}
		res = append(res, ow)
	}
	return res, nil
}

func (c *Connection) LoadAddressBook(ctx context.Context) error {
	res := make(map[core.Address]core.AddressInfo)
	var (
		addr   core.Address
		t      core.WalletType
		userID string
	)

	rows, err := c.client.Query(ctx, `
		SELECT address, type, user_id
		FROM payments.ton_wallets
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&addr, &t, &userID)
		if err != nil {
			return err
		}
		res[addr] = core.AddressInfo{Type: t, Owner: nil, UserID: userID}
	}

	rows, err = c.client.Query(ctx, `
		SELECT jw.address, jw.type, tw.address, jw.user_id
		FROM payments.jetton_wallets jw
		LEFT JOIN payments.ton_wallets tw ON jw.subwallet_id = tw.subwallet_id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var owner core.Address
		err = rows.Scan(&addr, &t, &owner, &userID)
		if err != nil {
			return err
		}
		res[addr] = core.AddressInfo{Type: t, Owner: &owner, UserID: userID}
	}

	c.addressBook.addresses = res
	log.Info("Address book loaded")
	return nil
}

func saveExternalIncome(ctx context.Context, tx pgx.Tx, inc core.ExternalIncome) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO payments.external_incomes (
		lt,
		utime,
		deposit_address,
		payer_address,
		amount,
		comment,
		payer_workchain,
		tx_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)                                               
	`,
		inc.Lt,
		time.Unix(int64(inc.Utime), 0),
		inc.To,
		inc.From,
		inc.Amount,
		inc.Comment,
		inc.FromWorkchain,
		inc.TxHash,
	)
	return err
}

func (c *Connection) saveInternalIncome(ctx context.Context, tx pgx.Tx, inc core.InternalIncome) error {
	memo, err := uuid.FromString(inc.Memo)
	if err != nil {
		return err
	}

	wType, ok := c.GetWalletType(inc.From)
	var from core.Address
	if ok && wType == core.JettonOwner { // convert jetton owner address to jetton wallet address
		err = tx.QueryRow(ctx, `
			SELECT jw.address
			FROM payments.ton_wallets tw
			LEFT JOIN payments.jetton_wallets jw ON tw.subwallet_id = jw.subwallet_id
			WHERE tw.address = $1
		`, inc.From).Scan(&from)
		if err != nil {
			return err
		}
	} else {
		from = inc.From
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO payments.internal_incomes (
		lt,
		utime,
		deposit_address,
		amount,
		memo)
		VALUES ($1, $2, $3, $4, $5)                                               
	`,
		inc.Lt,
		time.Unix(int64(inc.Utime), 0),
		from,
		inc.Amount,
		memo,
	)
	return err
}

func (c *Connection) SaveWithdrawalRequest(ctx context.Context, w core.WithdrawalRequest) (int64, error) {
	var queryID int64
	err := c.client.QueryRow(ctx, `
		INSERT INTO payments.withdrawal_requests (
		user_id,
		user_query_id,
		amount,
		currency,
		bounceable,
		dest_address,
		comment,
		is_internal
	)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING query_id
	`,
		w.UserID,
		w.QueryID,
		w.Amount,
		w.Currency,
		w.Bounceable,
		w.Destination,
		w.Comment,
		w.IsInternal,
	).Scan(&queryID)
	return queryID, err
}

func (c *Connection) SaveServiceWithdrawalRequest(ctx context.Context, w core.ServiceWithdrawalRequest) (
	uuid.UUID,
	error,
) {
	var memo uuid.UUID
	err := c.client.QueryRow(ctx, `
		INSERT INTO payments.service_withdrawal_requests (
		from_address,
		jetton_master		
	)
		VALUES ($1, $2)
		RETURNING memo
	`,
		w.From,
		w.JettonMaster,
	).Scan(&memo)
	return memo, err
}

func (c *Connection) UpdateServiceWithdrawalRequest(
	ctx context.Context,
	t core.ServiceWithdrawalTask,
	tonAmount core.Coins,
	expiredAt time.Time,
	filled bool,
) error {
	_, err := c.client.Exec(ctx, `
			UPDATE payments.service_withdrawal_requests
			SET
			    ton_amount = $1,
			    jetton_amount = $2,
		    	processed = not $4,
		    	expired_at = $3,
		    	filled = $4
			WHERE  memo = $5
		`, tonAmount, t.JettonAmount, expiredAt, filled, t.Memo)
	return err
}

func (c *Connection) IsWithdrawalRequestUnique(ctx context.Context, w core.WithdrawalRequest) (bool, error) {
	var queryID int64
	err := c.client.QueryRow(ctx, `
		SELECT query_id
		FROM payments.withdrawal_requests
		WHERE user_id = $1 AND user_query_id = $2 AND is_internal = false 
	`,
		w.UserID,
		w.QueryID,
	).Scan(&queryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}

func (c *Connection) GetExternalWithdrawalTasks(ctx context.Context, limit int) ([]core.ExternalWithdrawalTask, error) {
	var res []core.ExternalWithdrawalTask
	rows, err := c.client.Query(ctx, `
		SELECT DISTINCT ON (dest_address) dest_address,
		                                  query_id,
		                                  currency,
		                                  bounceable,
		                                  comment,
		                                  amount
		FROM   payments.withdrawal_requests
		WHERE  processing = false
        ORDER BY dest_address, query_id
		LIMIT  $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var w core.ExternalWithdrawalTask
		err = rows.Scan(&w.Destination, &w.QueryID, &w.Currency, &w.Bounceable, &w.Comment, &w.Amount)
		if err != nil {
			return nil, err
		}
		res = append(res, w)
	}
	return res, nil
}

// GetServiceHotWithdrawalTasks return tasks for Hot wallet withdrawals
func (c *Connection) GetServiceHotWithdrawalTasks(ctx context.Context, limit int) ([]core.ServiceWithdrawalTask, error) {
	var tasks []core.ServiceWithdrawalTask
	rows, err := c.client.Query(ctx, `
		SELECT DISTINCT ON (from_address) swr.from_address,
		                                  swr.memo,
		                                  swr.jetton_master,
		                                  tw.subwallet_id
		FROM   payments.service_withdrawal_requests swr
		LEFT JOIN payments.ton_wallets tw ON swr.from_address = tw.address
		WHERE  processed = false and type = ANY($1) and filled = false
        ORDER BY from_address
		LIMIT  $2
	`, []core.WalletType{core.JettonOwner, core.TonDepositWallet}, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var w core.ServiceWithdrawalTask
		err = rows.Scan(&w.From, &w.Memo, &w.JettonMaster, &w.SubwalletID)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, w)
	}
	return tasks, nil
}

// GetServiceDepositWithdrawalTasks return tasks for TON deposit wallets
func (c *Connection) GetServiceDepositWithdrawalTasks(ctx context.Context, limit int) ([]core.ServiceWithdrawalTask, error) {
	var tasks []core.ServiceWithdrawalTask
	rows, err := c.client.Query(ctx, `
		SELECT DISTINCT ON (from_address) swr.from_address,
		                                  swr.memo,
		                                  swr.jetton_master,
		                                  swr.jetton_amount,
		                                  tw.subwallet_id
		FROM   payments.service_withdrawal_requests swr
		LEFT JOIN payments.ton_wallets tw ON swr.from_address = tw.address
		WHERE  processed = false AND filled = true AND type = $1
        ORDER BY from_address
		LIMIT $2
	`, core.TonDepositWallet, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var w core.ServiceWithdrawalTask
		err = rows.Scan(&w.From, &w.Memo, &w.JettonMaster, &w.JettonAmount, &w.SubwalletID)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, w)
	}
	return tasks, nil
}

func saveBlock(ctx context.Context, tx pgx.Tx, block core.ShardBlockHeader) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO payments.block_data (
		shard,
		seqno,
		root_hash,
		file_hash,
		gen_utime                                 
		) VALUES ($1, $2, $3, $4, $5)                                               
	`,
		block.Shard,
		block.SeqNo,
		block.RootHash,
		block.FileHash,
		time.Unix(int64(block.GenUtime), 0),
	)
	return err
}

func updateInternalWithdrawal(ctx context.Context, tx pgx.Tx, w core.InternalWithdrawal) error {
	memo, err := uuid.FromString(w.Memo)
	if err != nil {
		return err
	}

	var (
		sendingLt     *int64
		alreadyFailed bool
	)

	err = tx.QueryRow(ctx, `
		SELECT failed, sending_lt
		FROM payments.internal_withdrawals
		WHERE  memo = $1
	`, memo).Scan(&alreadyFailed, &sendingLt)

	if alreadyFailed {
		audit.Log(audit.Error, "internal withdrawal message", core.InternalWithdrawalEvent,
			fmt.Sprintf("successful withdrawal for expired internal withdrawal message. memo: %v", w.Memo))
		return fmt.Errorf("invalid behavior of the expiration processor")
	}

	if sendingLt == nil {
		audit.Log(audit.Error, "internal withdrawal message", core.InternalWithdrawalEvent,
			fmt.Sprintf("successful withdrawal without sending confirmation. memo: %v", w.Memo))
		return fmt.Errorf("invalid event order")
	}

	if w.IsFailed {
		_, err = tx.Exec(ctx, `
			UPDATE payments.internal_withdrawals
			SET
		    	failed = true
			WHERE  memo = $1
		`, memo)
		return err
	}
	_, err = tx.Exec(ctx, `
			UPDATE payments.internal_withdrawals
			SET
		    	finish_lt = $1,
		    	finished_at = $2,
		    	amount = amount + $3
			WHERE  memo = $4
		`, w.Lt, time.Unix(int64(w.Utime), 0), w.Amount, memo)
	return err
}

func (c *Connection) SaveInternalWithdrawalTask(
	ctx context.Context,
	task core.InternalWithdrawalTask,
	expiredAt time.Time,
	memo uuid.UUID,
) error {
	_, err := c.client.Exec(ctx, `
		INSERT INTO payments.internal_withdrawals (
		since_lt,
		from_address,
		expired_at,
		memo
		) VALUES ($1, $2, $3, $4)
	`,
		task.Lt,
		task.From,
		expiredAt,
		memo,
	)
	return err
}

func (c *Connection) SaveParsedBlockData(ctx context.Context, events core.BlockEvents) error {
	tx, err := c.client.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, ei := range events.ExternalIncomes {
		err = saveExternalIncome(ctx, tx, ei)
		if err != nil {
			return err
		}
	}
	for _, ii := range events.InternalIncomes {
		err = c.saveInternalIncome(ctx, tx, ii)
		if err != nil {
			return err
		}
	}
	for _, sc := range events.SendingConfirmations {
		err = applySendingConfirmations(ctx, tx, sc)
		if err != nil {
			return err
		}
	}
	for _, iw := range events.InternalWithdrawals {
		err = updateInternalWithdrawal(ctx, tx, iw)
		if err != nil {
			return err
		}
	}
	for _, ew := range events.ExternalWithdrawals {
		err = updateExternalWithdrawal(ctx, tx, ew)
		if err != nil {
			return err
		}
	}
	for _, wc := range events.WithdrawalConfirmations {
		err = applyJettonWithdrawalConfirmation(ctx, tx, wc)
		if err != nil {
			return err
		}
	}
	err = saveBlock(ctx, tx, events.Block)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	return err
}

func (c *Connection) GetTonInternalWithdrawalTasks(ctx context.Context, limit int) ([]core.InternalWithdrawalTask, error) {
	var tasks []core.InternalWithdrawalTask
	// lt > finish_lt condition because all TONs withdraws
	rows, err := c.client.Query(ctx, `
		SELECT deposit_address, MAX(lt) AS last_lt, tw.subwallet_id
		FROM payments.external_incomes di
			LEFT JOIN (
			SELECT from_address, since_lt, finish_lt
			FROM payments.internal_withdrawals iw1
			WHERE since_lt = (
				SELECT MAX(iw2.since_lt)
				FROM payments.internal_withdrawals iw2
				WHERE iw2.from_address = iw1.from_address AND failed = false
			)
		) as iw3 ON from_address = deposit_address
		JOIN payments.ton_wallets tw ON di.deposit_address = tw.address
		WHERE ((since_lt IS NOT NULL AND finish_lt IS NOT NULL AND lt > finish_lt) OR (since_lt IS NULL)) 
		  AND type = $1
		GROUP BY deposit_address, tw.subwallet_id
		LIMIT $2
	`, core.TonDepositWallet, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var task core.InternalWithdrawalTask
		err = rows.Scan(&task.From, &task.Lt, &task.SubwalletID)
		if err != nil {
			return nil, err
		}
		task.Currency = core.TonSymbol
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (c *Connection) GetJettonInternalWithdrawalTasks(
	ctx context.Context,
	forbiddenAddresses []core.Address,
	limit int,
) (
	[]core.InternalWithdrawalTask, error,
) {
	var tasks []core.InternalWithdrawalTask
	excludedAddr := make([][]byte, 0) // it is important for 'deposit_address = ANY($2)' sql constraint
	for _, a := range forbiddenAddresses {
		excludedAddr = append(excludedAddr, a[:]) // array of core.Address not supported by driver
	}
	rows, err := c.client.Query(ctx, `
		SELECT deposit_address, MAX(lt) AS last_lt, jw.subwallet_id, jw.currency
		FROM payments.external_incomes di
			LEFT JOIN (
			SELECT from_address, since_lt, finish_lt
			FROM payments.internal_withdrawals iw1
			WHERE since_lt = (
				SELECT MAX(iw2.since_lt)
				FROM payments.internal_withdrawals iw2
				WHERE iw2.from_address = iw1.from_address AND failed = false
			)
		) as iw3 ON from_address = deposit_address
		JOIN payments.jetton_wallets jw ON di.deposit_address = jw.address
		LEFT JOIN payments.ton_wallets tw ON jw.subwallet_id = tw.subwallet_id
		WHERE ((since_lt IS NOT NULL AND lt > since_lt AND finish_lt IS NOT NULL)
		   OR (since_lt IS NULL)) AND jw.type = $1 AND NOT tw.address = ANY($2)
		GROUP BY deposit_address, jw.subwallet_id, jw.currency
		LIMIT $3
	`, core.JettonDepositWallet, excludedAddr, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var task core.InternalWithdrawalTask
		err = rows.Scan(&task.From, &task.Lt, &task.SubwalletID, &task.Currency)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func applyJettonWithdrawalConfirmation(
	ctx context.Context,
	tx pgx.Tx,
	confirm core.JettonWithdrawalConfirmation,
) error {
	_, err := tx.Exec(ctx, `
			UPDATE payments.external_withdrawals
			SET
		    	confirmed = true
			WHERE  query_id = $1 AND processed_lt IS NOT NULL
		`, confirm.QueryId)
	return err
}

func updateExternalWithdrawal(ctx context.Context, tx pgx.Tx, w core.ExternalWithdrawal) error {
	var queryID int64

	var alreadyFailed bool
	err := tx.QueryRow(ctx, `
		SELECT failed
		FROM payments.external_withdrawals
		WHERE  msg_uuid = $1 AND address = $2
	`, w.ExtMsgUuid, w.To).Scan(&alreadyFailed)
	if alreadyFailed {
		audit.Log(audit.Error, "external withdrawal message", core.ExternalWithdrawalEvent,
			fmt.Sprintf("successful withdrawal for expired external withdrawal message. msg uuid: %v", w.ExtMsgUuid.String()))
		return fmt.Errorf("invalid behavior of the expiration processor")
	}

	// if there was a transaction on the hot wallet but the message was not sent
	// set processed = true to prevent burn balance
	if w.IsFailed {
		err := tx.QueryRow(ctx, `
			UPDATE payments.external_withdrawals
			SET
		    	failed = true,
		    	tx_hash = $1
			WHERE  msg_uuid = $2 AND address = $3
			RETURNING query_id
		`, w.TxHash, w.ExtMsgUuid, w.To).Scan(&queryID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			UPDATE payments.withdrawal_requests
			SET
		    	processed = true
			WHERE  query_id = $1
		`, queryID)
		return err
	}

	err = tx.QueryRow(ctx, `
			UPDATE payments.external_withdrawals
			SET
		    	processed_lt = $1,
		    	processed_at = $2,
		    	tx_hash = $3
			WHERE  msg_uuid = $4 AND address = $5
			RETURNING query_id
		`, w.Lt, time.Unix(int64(w.Utime), 0), w.TxHash, w.ExtMsgUuid, w.To).Scan(&queryID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
			UPDATE payments.withdrawal_requests
			SET
		    	processed = true
			WHERE  query_id = $1
		`, queryID)
	return err
}

func applySendingConfirmations(ctx context.Context, tx pgx.Tx, w core.SendingConfirmation) error {
	var alreadyFailed bool
	memo, err := uuid.FromString(w.Memo)
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx, `
			UPDATE payments.internal_withdrawals
			SET
		    	sending_lt = $1
			WHERE  memo = $2
			RETURNING failed
		`, w.Lt, memo).Scan(&alreadyFailed)
	if err != nil {
		return err
	}
	if alreadyFailed {
		audit.Log(audit.Error, "internal withdrawal message", core.InternalWithdrawalEvent,
			fmt.Sprintf("successful sending for expired internal withdrawal message. memo: %v", w.Memo))
		return fmt.Errorf("invalid behavior of the expiration processor")
	}
	return err
}

func (c *Connection) CreateExternalWithdrawals(
	ctx context.Context,
	tasks []core.ExternalWithdrawalTask,
	extMsgUuid uuid.UUID,
	expiredAt time.Time,
) error {
	tx, err := c.client.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, t := range tasks {
		_, err = tx.Exec(ctx, `
			INSERT INTO payments.external_withdrawals (
				msg_uuid,
				query_id,
				expired_at,
				address
			) VALUES ($1, $2, $3, $4)                                               
		`, extMsgUuid, t.QueryID, expiredAt, t.Destination)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			UPDATE payments.withdrawal_requests
			SET
		    	processing = true		    	
			WHERE  query_id = $1
		`, t.QueryID)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (c *Connection) GetTonHotWalletAddress(ctx context.Context) (core.Address, error) {
	var addr core.Address
	err := c.client.QueryRow(ctx, `
		SELECT address 
		FROM payments.ton_wallets
		WHERE TYPE = $1
	`, core.TonHotWallet).Scan(&addr)
	if errors.Is(err, pgx.ErrNoRows) {
		err = core.ErrNotFound
	}
	return addr, err
}

func (c *Connection) GetLastSavedBlockID(ctx context.Context) (*ton.BlockIDExt, error) {
	var blockID ton.BlockIDExt
	err := c.client.QueryRow(ctx, `
		SELECT 
		    seqno, 
		    shard, 
		    root_hash, 
		    file_hash
		FROM payments.block_data
		ORDER BY seqno DESC
		LIMIT 1
	`).Scan(
		&blockID.SeqNo,
		&blockID.Shard,
		&blockID.RootHash,
		&blockID.FileHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	blockID.Workchain = core.DefaultWorkchain
	return &blockID, nil
}

// SetExpired TODO: maybe add block related expiration
func (c *Connection) SetExpired(ctx context.Context) error {
	_, err := c.client.Exec(ctx, `
			UPDATE payments.internal_withdrawals
			SET
		    	failed = true		    	
			WHERE  expired_at < $1 AND sending_lt IS NULL AND failed = false
	`, time.Now().Add(-config.AllowableBlockchainLagging))
	if err != nil {
		return err
	}

	tx, err := c.client.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// processed_lt IS NULL AND failed = false - for lost external messages
	rows, err := tx.Query(ctx, `
			UPDATE payments.external_withdrawals
			SET
		    	failed = true		    	
			WHERE  expired_at < $1 AND processed_lt IS NULL AND failed = false
			RETURNING query_id
	`, time.Now().Add(-config.AllowableBlockchainLagging))

	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var queryID int64
		err = rows.Scan(&queryID)
		if err != nil {
			return err
		}
		ids = append(ids, queryID)
	}

	for _, id := range ids {
		_, err = tx.Exec(ctx, `
			UPDATE payments.withdrawal_requests
			SET
		    	processing = false
			WHERE  query_id = $1
		`, id)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (c *Connection) IsActualBlockData(ctx context.Context) (bool, error) {
	var lastBlockTime time.Time
	err := c.client.QueryRow(ctx, `
		SELECT 
		    gen_utime
		FROM payments.block_data
		ORDER BY seqno DESC
		LIMIT 1
	`).Scan(&lastBlockTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return time.Since(lastBlockTime) < config.AllowableBlockchainLagging, nil
}

func (c *Connection) IsInProgressInternalWithdrawalRequest(
	ctx context.Context,
	dest core.Address,
	currency string,
) (
	bool,
	error,
) {
	var queryID int64
	err := c.client.QueryRow(ctx, `
		SELECT query_id
		FROM payments.withdrawal_requests
		WHERE dest_address = $1 AND 
		      currency = $2 AND 
		      is_internal = true AND
		      processed = false
		LIMIT 1
	`,
		dest,
		currency,
	).Scan(&queryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetExternalWithdrawalStatus returns status and hash of transaction for external withdrawal
func (c *Connection) GetExternalWithdrawalStatus(ctx context.Context, id int64) (core.WithdrawalStatus, []byte, error) {
	var processing, processed bool
	err := c.client.QueryRow(ctx, `
		SELECT processing, processed
		FROM payments.withdrawal_requests
		WHERE query_id = $1 AND is_internal = false
		LIMIT 1
	`, id).Scan(&processing, &processed)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, core.ErrNotFound
	}
	if err != nil {
		return "", nil, err
	}
	if processing && processed {
		var (
			txHash   []byte
			isFailed bool
		)
		// must be only one record
		err = c.client.QueryRow(ctx, `
		SELECT tx_hash, failed
		FROM payments.external_withdrawals
		WHERE query_id = $1 AND tx_hash IS NOT NULL
		LIMIT 1
	`, id).Scan(&txHash, &isFailed)
		if err != nil {
			return "", nil, err
		}
		if isFailed {
			return core.FailedStatus, txHash, nil
		}
		return core.ProcessedStatus, txHash, nil
	}
	if processing && !processed {
		return core.ProcessingStatus, nil, nil
	}
	if !processing && !processed {
		return core.PendingStatus, nil, nil
	}
	return "", nil, fmt.Errorf("bad status")
}

// GetIncome returns list of incomes by user_id
func (c *Connection) GetIncome(
	ctx context.Context,
	userID string,
	isDepositSide bool,
) (
	[]core.TotalIncome,
	error,
) {
	var sqlStatement string
	if isDepositSide {
		sqlStatement = `
			SELECT COALESCE(jw.address,tw.address) as deposit, COALESCE(SUM(i.amount),0) as balance, COALESCE(jw.currency,$1) as currency
			FROM payments.ton_wallets tw
         		LEFT JOIN payments.jetton_wallets jw ON jw.subwallet_id = tw.subwallet_id
         		LEFT JOIN payments.external_incomes i ON i.deposit_address = COALESCE(jw.address,tw.address)
			WHERE tw.user_id = $2 AND tw.type = ANY($3)
			GROUP BY deposit, tw.address, jw.currency
		`
	} else {
		sqlStatement = `
			SELECT COALESCE(jw.address,tw.address) as deposit, COALESCE(SUM(i.amount),0) as balance, COALESCE(jw.currency,$1) as currency
			FROM payments.ton_wallets tw
         		LEFT JOIN payments.jetton_wallets jw ON jw.subwallet_id = tw.subwallet_id
         		LEFT JOIN payments.internal_incomes i ON i.deposit_address = COALESCE(jw.address,tw.address)
			WHERE tw.user_id = $2 AND tw.type = ANY($3)
			GROUP BY deposit, tw.address, jw.currency
		`
	}

	rows, err := c.client.Query(
		ctx,
		sqlStatement,
		core.TonSymbol,
		userID,
		[]core.WalletType{core.TonDepositWallet, core.JettonOwner},
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]core.TotalIncome, 0)
	for rows.Next() {
		var deposit core.TotalIncome
		err = rows.Scan(&deposit.Deposit, &deposit.Amount, &deposit.Currency)
		if err != nil {
			return nil, err
		}
		res = append(res, deposit)
	}
	return res, nil
}

// GetIncomeHistory returns list of external incomes for deposit side by user_id and currency
func (c *Connection) GetIncomeHistory(
	ctx context.Context,
	userID string,
	currency string,
	limit int,
	offset int,
	ascOrder bool,
) (
	[]core.ExternalIncome,
	error,
) {
	var (
		res          []core.ExternalIncome
		sqlStatement string
		walletType   core.WalletType
	)

	order := "DESC"
	if ascOrder {
		order = "ASC"
	}

	if currency == core.TonSymbol {
		sqlStatement = fmt.Sprintf(`
			SELECT utime, lt, payer_address, deposit_address, amount, comment, payer_workchain, tx_hash
			FROM payments.external_incomes i
				LEFT JOIN payments.ton_wallets tw ON i.deposit_address = tw.address
			WHERE tw.type = $1 AND tw.user_id = $2 AND $3 = $3
			ORDER BY lt %s
			LIMIT $4
			OFFSET $5
		`, order)
		walletType = core.TonDepositWallet
	} else {
		sqlStatement = fmt.Sprintf(`
			SELECT utime, lt, payer_address, deposit_address, amount, comment, payer_workchain, tx_hash
			FROM payments.external_incomes i
			    LEFT JOIN payments.jetton_wallets jw ON i.deposit_address = jw.address
			WHERE jw.type = $1 AND jw.user_id = $2 AND jw.currency = $3
			ORDER BY lt %s
			LIMIT $4
			OFFSET $5
		`, order)
		walletType = core.JettonDepositWallet
	}

	rows, err := c.client.Query(ctx, sqlStatement, walletType, userID, currency, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			income core.ExternalIncome
			t      time.Time
		)
		err = rows.Scan(&t, &income.Lt, &income.From, &income.To, &income.Amount, &income.Comment, &income.FromWorkchain, &income.TxHash)
		if err != nil {
			return nil, err
		}
		income.Utime = uint32(t.Unix())
		res = append(res, income)
	}
	return res, nil
}

// GetIncomeByTx returns external income and currency for deposit side by transaction hash
func (c *Connection) GetIncomeByTx(
	ctx context.Context,
	txHash []byte,
) (
	*core.ExternalIncome,
	string,
	error,
) {

	var (
		income core.ExternalIncome
		t      time.Time
	)

	err := c.client.QueryRow(ctx, `
		SELECT utime, lt, payer_address, deposit_address, amount, comment, payer_workchain
		FROM payments.external_incomes i
		LEFT JOIN payments.ton_wallets tw ON i.deposit_address = tw.address
		WHERE tw.type = $1 AND i.tx_hash = $2
		LIMIT 1
	`, core.TonDepositWallet, txHash).Scan(&t, &income.Lt, &income.From, &income.To, &income.Amount, &income.Comment, &income.FromWorkchain)
	if errors.Is(err, pgx.ErrNoRows) {
		var currency string
		// amount > 0 means receiving an aggregated transaction for an unidentified jetton replenishment
		err = c.client.QueryRow(ctx, `
			SELECT utime, lt, payer_address, deposit_address, amount, comment, payer_workchain, jw.currency
			FROM payments.external_incomes i
			LEFT JOIN payments.jetton_wallets jw ON i.deposit_address = jw.address
			WHERE jw.type = $1 AND i.tx_hash = $2 AND i.amount > 0
			LIMIT 1
		`, core.JettonDepositWallet, txHash).Scan(&t, &income.Lt, &income.From, &income.To, &income.Amount, &income.Comment, &income.FromWorkchain, &currency)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", core.ErrNotFound // not found
		}
		if err != nil {
			return nil, "", err
		}
		income.Utime = uint32(t.Unix())
		income.TxHash = txHash
		return &income, currency, nil
	}
	if err != nil {
		return nil, "", err
	}
	income.Utime = uint32(t.Unix())
	income.TxHash = txHash
	return &income, core.TonSymbol, nil
}
