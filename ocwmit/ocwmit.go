// Package ocwmit is the library behind the ocwmit command line:
// the HTTP client, request shaping, and typed data models for MIT OpenCourseWare
// courses fetched from the OCW sitemap.
package ocwmit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to MIT OCW.
const DefaultUserAgent = "Mozilla/5.0 (compatible; ocwmit-cli/0.1; +https://github.com/tamnd/ocwmit-cli)"

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults for the OCW sitemap.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://ocw.mit.edu",
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   60 * time.Second,
	}
}

// Client talks to MIT OpenCourseWare over HTTP.
type Client struct {
	cfg        Config
	httpClient *http.Client
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		b, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return b, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get: %w", lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
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
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

var (
	termRe      = regexp.MustCompile(`-(spring|fall|summer|january|iap|winter)-\d{4}$`)
	numBoundRe  = regexp.MustCompile(`-[a-z]`)
	courseLocRe = regexp.MustCompile(`<loc>https://ocw\.mit\.edu/courses/([^<]+)/sitemap\.xml</loc>`)
)

func parseSlug(slug string) (number, title, term string) {
	titleNum := slug
	if m := termRe.FindStringIndex(slug); m != nil {
		term = slug[m[0]+1:] // skip leading dash
		titleNum = slug[:m[0]]
	}
	if m := numBoundRe.FindStringIndex(titleNum); m != nil {
		number = strings.ReplaceAll(titleNum[:m[0]], "-", ".")
		rawTitle := strings.ReplaceAll(titleNum[m[0]+1:], "-", " ")
		title = toTitleCase(rawTitle)
	} else {
		number = strings.ReplaceAll(titleNum, "-", ".")
		title = ""
	}
	return
}

func toTitleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func parseCourses(xml string, limit int) []Course {
	matches := courseLocRe.FindAllStringSubmatch(xml, -1)
	var out []Course
	rank := 0
	for _, m := range matches {
		slug := m[1]
		number, title, term := parseSlug(slug)
		rank++
		if limit > 0 && rank > limit {
			break
		}
		out = append(out, Course{
			Rank:   rank,
			Number: number,
			Title:  title,
			Term:   term,
			Slug:   slug,
			URL:    "https://ocw.mit.edu/courses/" + slug + "/",
		})
	}
	return out
}

// List fetches all OCW courses from the sitemap and returns up to limit records.
// limit=0 returns all courses.
func (c *Client) List(ctx context.Context, limit int) ([]Course, error) {
	raw, err := c.get(ctx, c.cfg.BaseURL+"/sitemap.xml")
	if err != nil {
		return nil, err
	}
	return parseCourses(string(raw), limit), nil
}

// Search fetches all courses and filters client-side by query (title, number, or term).
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Course, error) {
	all, err := c.List(ctx, 0)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []Course
	rank := 0
	for _, course := range all {
		if strings.Contains(strings.ToLower(course.Title), q) ||
			strings.Contains(strings.ToLower(course.Number), q) ||
			strings.Contains(strings.ToLower(course.Term), q) {
			rank++
			course.Rank = rank
			out = append(out, course)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
