package queue

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
)

type AmqpClient struct {
	connection   *amqp.Connection
	channel      *amqp.Channel
	exchange     string
	enabled      bool
	Subscription <-chan amqp.Delivery
}

// NewAmqpClient creates new AMQP client and declare new exchange
func NewAmqpClient(uri string, queueName string) (*AmqpClient, error) {
	if !enabled {
		return &AmqpClient{enabled: false}, nil
	}
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	client := &AmqpClient{connection: conn, channel: ch, enabled: enabled}
	err = client.declareExchange(queueName)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (c *AmqpClient) declareExchange(exchangeName string) error {
	err := c.channel.ExchangeDeclare(
		exchangeName,
		"fanout",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}
	c.exchange = exchangeName
	return nil
}

// Publish publishes any payload to queue
func (c *AmqpClient) Publish(payload any) error {
	if !c.enabled {
		return nil
	}
	if c.exchange == "" {
		return fmt.Errorf("exchange not init")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	err = c.channel.Publish(
		c.exchange,
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	return err
}
