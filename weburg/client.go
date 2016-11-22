package weburg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	defaultUserAgent = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:27.0) Gecko/20100101 Firefox/27.0"
)

var (
	NoAuthCookie = http.Cookie{
		Name:  "session_id",
		Value: "noauth",
	}
)

type Client struct {
	client    *http.Client
	UserAgent string
	DebugHTTP bool
	Timeout   time.Duration
}

func NewClient(httpClient *http.Client) *Client {
	c := &Client{
		client:    httpClient,
		UserAgent: defaultUserAgent,
		Timeout:   10 * time.Second,
	}
	return c
}

func (c *Client) NewRequest(method, u string, body interface{}) (*http.Request, error) {
	buf := new(bytes.Buffer)
	if body != nil {
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, u, buf)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&NoAuthCookie)
	req.Header.Add("Content-Type", "text/html; charset=utf-8")
	req.Header.Add("X-Requested-With", "XMLHttpRequest")
	req.Header.Add("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Add("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) Do(req *http.Request) ([]byte, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Do error: %v", err)
	}
	defer resp.Body.Close()
	err = checkResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("Response error: %v", err)
	}
	data, err := ioutil.ReadAll(resp.Body)
	return data, err
}

func checkResponse(r *http.Response) error {
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}
	return fmt.Errorf("Incorrent response code: %d", r.StatusCode)
}
