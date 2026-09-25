// Package pixiv is the library behind the pixiv command: the HTTP client,
// request shaping, and the typed data models for Pixiv.
//
// The ranking API is open and requires no authentication. A browser-like
// User-Agent and a Referer header pointing at pixiv.net are required.
package pixiv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	pixivReferer = "https://www.pixiv.net/"
)

// DefaultUserAgent identifies the client to Pixiv.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Config holds constructor parameters.
type Config struct {
	BaseURL    string
	DicBaseURL string
	UserAgent  string
	Rate       time.Duration
	Retries    int
	Timeout    time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:    BaseURL,
		DicBaseURL: DicBaseURL,
		UserAgent:  DefaultUserAgent,
		Rate:       200 * time.Millisecond,
		Retries:    3,
		Timeout:    30 * time.Second,
	}
}

// Client talks to the Pixiv ranking API and the Pixiv encyclopedia.
type Client struct {
	httpClient *http.Client
	userAgent  string
	rate       time.Duration
	retries    int
	baseURL    string
	dicBaseURL string
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	if cfg.DicBaseURL == "" {
		cfg.DicBaseURL = DicBaseURL
	}
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		userAgent:  cfg.UserAgent,
		rate:       cfg.Rate,
		retries:    cfg.Retries,
		baseURL:    cfg.BaseURL,
		dicBaseURL: cfg.DicBaseURL,
	}
}

// Ranking fetches a ranked list of illustrations.
// mode: daily|weekly|monthly|rookie|original
// content: illust|manga|ugoira
// page: 1-based page number
// limit: max records to return (0 = all on page, up to 50)
func (c *Client) Ranking(ctx context.Context, mode, content string, page, limit int) ([]Illust, error) {
	rawURL := fmt.Sprintf("%s/ranking.php?mode=%s&content=%s&format=json&p=%d",
		c.baseURL, mode, content, page)

	body, err := c.get(ctx, rawURL, "application/json", pixivReferer)
	if err != nil {
		return nil, err
	}

	var resp wireResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode ranking response: %w", err)
	}

	items := resp.Contents
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	out := make([]Illust, len(items))
	for i, w := range items {
		out[i] = wireToIllust(w)
	}
	return out, nil
}

// HTTPError is a non-2xx response from Pixiv, kept typed so callers can map it
// onto the kit error taxonomy.
type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http %d: %s", e.StatusCode, e.URL)
}

// get fetches a URL with pacing, the given Accept and Referer headers, and
// retries on transient failures.
func (c *Client) get(ctx context.Context, rawURL, accept, referer string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL, accept, referer)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL, accept, referer string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", accept)
	req.Header.Set("Referer", referer)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, &HTTPError{StatusCode: resp.StatusCode, URL: rawURL}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, &HTTPError{StatusCode: resp.StatusCode, URL: rawURL}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		return 5 * time.Second
	}
	return d
}
