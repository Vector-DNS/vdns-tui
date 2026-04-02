package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	defaultTimeout  = 15 * time.Second
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
	userAgent       = "vdns-tui"
	basePath        = "/api/v1/user"
)

type Client struct {
	httpClient *http.Client
	server     string
	apiKey     string

	LastRateLimit *RateLimit
}

func New(server, apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		server:     server,
		apiKey:     apiKey,
	}
}

func (c *Client) DNSLookup(req DNSLookupRequest) (*DNSLookupResponse, error) {
	var resp DNSLookupResponse
	if err := c.post("/dns/lookup", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Propagation(req PropagationRequest) (*PropagationResponse, error) {
	var resp PropagationResponse
	if err := c.post("/dns/propagation", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Whois(req WhoisRequest) (*WhoisResponse, error) {
	var resp WhoisResponse
	if err := c.post("/whois/lookup", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Availability(req AvailabilityRequest) (*AvailabilityResponse, error) {
	var resp AvailabilityResponse
	if err := c.post("/availability/check", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) SSL(domain string) (*SSLResponse, error) {
	var resp SSLResponse
	path := "/ssl/check?domain=" + url.QueryEscape(domain)
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) post(path string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("could not serialize request: %w", err)
	}

	u := c.server + basePath + path
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req, result)
}

func (c *Client) get(path string, result any) error {
	u := c.server + basePath + path
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	return c.do(req, result)
}

func (c *Client) do(req *http.Request, result any) error {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	c.parseRateLimit(resp)

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("could not read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error string `json:"error"`
		}
		msg := string(body)
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error != "" {
			msg = apiErr.Error
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("could not parse response: %w", err)
		}
	}

	return nil
}

func (c *Client) parseRateLimit(resp *http.Response) {
	rl := &RateLimit{}

	if v := resp.Header.Get("X-RateLimit-Limit"); v != "" {
		rl.Limit, _ = strconv.Atoi(v)
	}
	if v := resp.Header.Get("X-RateLimit-Remaining"); v != "" {
		rl.Remaining, _ = strconv.Atoi(v)
	}
	if v := resp.Header.Get("Retry-After"); v != "" {
		rl.RetryAfter, _ = strconv.Atoi(v)
	}

	if rl.Limit > 0 {
		c.LastRateLimit = rl
	}
}
