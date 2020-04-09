package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/atsu/goat/health"
)

const (
	atsuEventEndpoint = "/slack/atsu-event"
	healthEndpoint    = "/health"
)

type Client struct {
	baseUrl string
}

func NewClient(baseUrl string) *Client {
	baseUrl = strings.TrimRight(baseUrl, "/")
	return &Client{baseUrl: baseUrl}
}

func (c *Client) SlackAtsuEvent(templateName string, fields map[string]string) error {
	jf, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid fields: %v", err)
	}
	u := fmt.Sprintf("%s%s?tpl=%s", c.baseUrl, atsuEventEndpoint, templateName)
	// TODO:(smt) validate body?
	_, err = http.Post(u, "application/json", bytes.NewBuffer(jf))
	if err != nil {
		return fmt.Errorf("post failed: %v", err)
	}
	return nil
}

func (c *Client) Health() (health.Event, error) {
	h := health.Event{}
	u := fmt.Sprintf("%s%s", c.baseUrl, healthEndpoint)
	res, err := http.Get(u)
	if err != nil {
		return h, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return h, errors.New("failed reading health")
	}
	if err := json.Unmarshal(body, &h); err != nil {
		return h, errors.New("failed reading health")
	}
	return h, nil
}
