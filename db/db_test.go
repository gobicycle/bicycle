package db

import (
	"context"
	"errors"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"os"
	"testing"
	"time"
)

var dbURI string

func migrateUp(source string) error {
	m, err := migrate.New("file://deploy/db", dbURI)
	if err != nil {
		return err
	}
	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	m, err = migrate.New("file://db/tests/"+source, dbURI)
	if err != nil {
		return err
	}
	defer m.Close()
	err = m.Force(-1)
	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func migrateDown(t *testing.T) {
	m, err := migrate.New("file://deploy/db", dbURI)
	if err != nil {
		t.Fatal("migrate down err: ", err)
	}
	err = m.Down()
	if err != nil {
		t.Fatal("migrate down err: ", err)
	}
	err = m.Drop()
	if err != nil {
		t.Fatal("migrate down err: ", err)
	}
	err1, err2 := m.Close()
	if err1 != nil || err2 != nil {
		t.Fatalf("migrate close err:\n %v\n %v\n", err1, err2)
	}
}

func init() {
	dbURI = os.Getenv("DB_URI")
	if dbURI == "" {
		panic("empty db uri var")
	}
}

func connect(t *testing.T) *Connection {
	c, err := NewConnection(dbURI)
	if err != nil {
		t.Fatal("connections err: ", err)
	}
	return c
}

func Test_NewConnection(t *testing.T) {
	connect(t)
}

func Test_GetTonInternalWithdrawalTasks(t *testing.T) {
	c := connect(t)
	source := "get-ton-internal-withdrawal-tasks"
	err := migrateUp(source)
	if err != nil {
		t.Fatal("migrate up err: ", err)
	}
	defer migrateDown(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	res, err := c.GetTonInternalWithdrawalTasks(ctx, 100)
	if err != nil {
		t.Fatal("get tasks err: ", err)
	}
	if len(res) != 1 {
		t.Fatal("only one task must be loaded")
	}
	if res[0].SubwalletID != 2 {
		t.Fatal("task must be loaded only for deposit A")
	}
	if res[0].Lt != 2 {
		t.Fatal("task must be loaded only for second payment")
	}
}

func Test_GetJettonInternalWithdrawalTasks(t *testing.T) {
	c := connect(t)
	source := "get-jetton-internal-withdrawal-tasks"
	err := migrateUp(source)
	if err != nil {
		t.Fatal("migrate up err: ", err)
	}
	defer migrateDown(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	res, err := c.GetJettonInternalWithdrawalTasks(ctx, 250)
	if err != nil {
		t.Fatal("get tasks err: ", err)
	}
	if len(res) != 2 {
		t.Fatal("two tasks must be loaded")
	}
	if res[0].SubwalletID != 2 || res[1].SubwalletID != 4 {
		t.Fatal("tasks must be loaded only for deposits A and C")
	}
	if res[0].Lt != 2 {
		t.Fatal("task must be loaded only for second payment for deposit A")
	}
	if res[1].Lt != 1 {
		t.Fatal("task must be loaded only for first payment for deposit C")
	}
}

func Test_SetExpired(t *testing.T) {
	type extResult struct {
		queryID    int
		failed     bool
		processing bool
		processed  bool
	}
	externalResult := [7]extResult{
		{1, true, false, false},
		{2, true, false, false},
		{3, true, false, false},
		{4, false, true, false},
		{5, true, false, false},
		{6, false, true, true},
		{7, false, true, true},
	}
	internalResult := [6]bool{true, true, false, true, false, false}

	c := connect(t)
	source := "set-expired"
	err := migrateUp(source)
	if err != nil {
		t.Fatal("migrate up err: ", err)
	}
	defer migrateDown(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	err = c.SetExpired(ctx)
	if err != nil {
		t.Fatal("set expired err: ", err)
	}

	// load external withdrawals data
	rows, err := c.client.Query(ctx, `
		SELECT ew.query_id, failed, wr.processing, wr.processed
		FROM   payments.external_withdrawals ew
		LEFT JOIN payments.withdrawal_requests wr ON wr.query_id = ew.query_id 
        ORDER BY ew.query_id ASC
	`)
	if err != nil {
		t.Fatal("get data err: ", err)
	}
	defer rows.Close()

	var (
		extRes [7]extResult
		i      = 0
	)
	for rows.Next() {
		var r extResult
		var err = rows.Scan(&r.queryID, &r.failed, &r.processing, &r.processed)
		if err != nil {
			t.Fatal("scan err: ", err)
		}
		extRes[i] = r
		i++
	}
	if externalResult != extRes {
		t.Fatalf("invalid external result pattern: %v", extRes)
	}

	// load internal withdrawals data
	rows, err = c.client.Query(ctx, `
		SELECT failed
		FROM   payments.internal_withdrawals
        ORDER BY since_lt ASC
	`)
	if err != nil {
		t.Fatal("get data err: ", err)
	}
	defer rows.Close()
	var intRes [6]bool
	i = 0
	for rows.Next() {
		var r bool
		var err = rows.Scan(&r)
		if err != nil {
			t.Fatal("scan err: ", err)
		}
		intRes[i] = r
		i++
	}
	if internalResult != intRes {
		t.Fatalf("invalid internal result pattern: %v", intRes)
	}
}
