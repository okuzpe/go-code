package llm

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Retry policy for LLM HTTP requests (D22): exponential backoff on rate limits / overload.
const (
	retryMaxAttempts = 10
	retryBaseDelay   = 500 * time.Millisecond
	retryMaxDelay    = 5 * time.Minute
)

func isRetryableHTTPStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// retryAfterFromResponse parses Retry-After (seconds). Returns 0 if missing or invalid.
func retryAfterFromResponse(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	s := resp.Header.Get("Retry-After")
	if s == "" {
		return 0
	}
	secs, err := strconv.Atoi(s)
	if err != nil || secs < 0 {
		return 0
	}
	d := time.Duration(secs) * time.Second
	if d > retryMaxDelay {
		return retryMaxDelay
	}
	return d
}

func backoffForAttempt(attempt int) time.Duration {
	d := retryBaseDelay
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= retryMaxDelay {
			return retryMaxDelay
		}
	}
	if d > retryMaxDelay {
		return retryMaxDelay
	}
	return d
}

func drainAndClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// doHTTPWithRetry runs client.Do until a non-retryable response or success, rebuilding the
// request each time so the body can be re-read. Caller must close resp.Body when done.
func doHTTPWithRetry(ctx context.Context, client *http.Client, build func() (*http.Request, error)) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	var lastErr error
	for attempt := 0; attempt < retryMaxAttempts; attempt++ {
		req, err := build()
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt == retryMaxAttempts-1 {
				return nil, err
			}
			if err := sleepForRetry(ctx, backoffForAttempt(attempt)); err != nil {
				return nil, err
			}
			continue
		}
		if isRetryableHTTPStatus(resp.StatusCode) {
			if attempt == retryMaxAttempts-1 {
				return resp, nil
			}
			wait := retryAfterFromResponse(resp)
			if wait <= 0 {
				wait = backoffForAttempt(attempt)
			}
			drainAndClose(resp)
			if err := sleepForRetry(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}
		return resp, nil
	}
	return nil, lastErr
}

// sleepForRetry is sleepCtx by default; tests may swap it to avoid real delays.
var sleepForRetry = sleepCtx

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
