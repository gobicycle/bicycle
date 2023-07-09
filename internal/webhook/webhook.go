package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type Client struct {
	client *http.Client
	uri    string
	token  string
}

// NewWebhookClient creates new webhook client
func NewWebhookClient(uri string, token string) (*Client, error) {
	if uri == "" {
		return nil, fmt.Errorf("emty uri")
	}
	if token == "" {
		log.Infof("empty token for webhook")
	}
	return &Client{
		client: &http.Client{Timeout: 10 * time.Second},
		uri:    uri,
		token:  token,
	}, nil
}

func (s *Client) Publish(payload any) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequest("POST", s.uri, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	if s.token != "" {
		request.Header.Add("Authorization", "Bearer "+s.token)
	}
	for i := 0; i < 3; i++ {
		err := send(s.client, request)
		if err != nil {
			log.Errorf("webhook sending error: %v", err)
			continue
		}
		return nil
	}
	return fmt.Errorf("attempts to send a webhook ended")
}

func send(client *http.Client, request *http.Request) error {
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("webhook sending error: %v", err)
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Fatalf("response body close error: %v", err)
		}
	}()
	if response.StatusCode == 200 {
		return nil
	} else {
		return fmt.Errorf("webhook response status: %v", response.Status)
	}
}
