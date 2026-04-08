package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBackoffForAttempt(t *testing.T) {
	if got := backoffForAttempt(0); got != retryBaseDelay {
		t.Fatalf("attempt 0: got %v want %v", got, retryBaseDelay)
	}
	if got := backoffForAttempt(1); got != 2*retryBaseDelay {
		t.Fatalf("attempt 1: got %v", got)
	}
	cap := backoffForAttempt(20)
	if cap != retryMaxDelay {
		t.Fatalf("capped backoff: got %v want %v", cap, retryMaxDelay)
	}
}

func TestRetryAfterFromResponse(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	if retryAfterFromResponse(resp) != 0 {
		t.Fatal("expected 0")
	}
	resp.Header.Set("Retry-After", "2")
	if got := retryAfterFromResponse(resp); got != 2*time.Second {
		t.Fatalf("got %v", got)
	}
}

func TestDoHTTPWithRetry_429ThenOK(t *testing.T) {
	prev := sleepForRetry
	sleepForRetry = func(ctx context.Context, d time.Duration) error { return nil }
	defer func() { sleepForRetry = prev }()

	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := n.Add(1)
		if c < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()
	resp, err := doHTTPWithRetry(ctx, client, func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "ok" {
		t.Fatalf("body %q", b)
	}
	if n.Load() != 3 {
		t.Fatalf("expected 3 requests, got %d", n.Load())
	}
}

func TestDoHTTPWithRetry_NonRetryable4xx(t *testing.T) {
	prev := sleepForRetry
	sleepForRetry = func(ctx context.Context, d time.Duration) error { return nil }
	defer func() { sleepForRetry = prev }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()
	resp, err := doHTTPWithRetry(ctx, client, func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestDoHTTPWithRetry_Last429Returned(t *testing.T) {
	prev := sleepForRetry
	sleepForRetry = func(ctx context.Context, d time.Duration) error { return nil }
	defer func() { sleepForRetry = prev }()

	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()
	resp, err := doHTTPWithRetry(ctx, client, func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("got %d", resp.StatusCode)
	}
	if n.Load() != int32(retryMaxAttempts) {
		t.Fatalf("expected %d attempts, got %d", retryMaxAttempts, n.Load())
	}
}

func TestIsRetryableHTTPStatus(t *testing.T) {
	if !isRetryableHTTPStatus(http.StatusTooManyRequests) {
		t.Fatal("429 should retry")
	}
	if isRetryableHTTPStatus(http.StatusBadRequest) {
		t.Fatal("400 should not retry")
	}
}

func TestDoHTTPWithRetry_RepostBody(t *testing.T) {
	prev := sleepForRetry
	sleepForRetry = func(ctx context.Context, d time.Duration) error { return nil }
	defer func() { sleepForRetry = prev }()

	var bodies []string
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		c := n.Add(1)
		if c < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	payload := `{"hello":"world"}`
	ctx := context.Background()
	resp, err := doHTTPWithRetry(ctx, srv.Client(), func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, strings.NewReader(payload))
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(bodies) != 2 || bodies[0] != payload || bodies[1] != payload {
		t.Fatalf("bodies: %#v", bodies)
	}
}
