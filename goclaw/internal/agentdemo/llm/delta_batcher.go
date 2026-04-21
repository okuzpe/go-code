package llm

import (
	"strings"
	"time"
)

// Default batching tuned like goclaw chat sink: fewer UI updates, smoother scroll.
const (
	DefaultDeltaBatchInterval = 42 * time.Millisecond
	DefaultDeltaBatchMaxBytes = 640
)

// StreamDeltaBatcher coalesces deltas on a single goroutine (the stream reader).
// It is not safe for concurrent use from multiple goroutines.
type StreamDeltaBatcher struct {
	interval time.Duration
	maxBytes int

	buf   strings.Builder
	timer *time.Timer

	flush func(chunk string)
}

// NewStreamDeltaBatcher wires flush to receive non-empty chunks after coalescing.
func NewStreamDeltaBatcher(interval time.Duration, maxBytes int, flush func(chunk string)) *StreamDeltaBatcher {
	if interval <= 0 {
		interval = DefaultDeltaBatchInterval
	}
	if maxBytes <= 0 {
		maxBytes = DefaultDeltaBatchMaxBytes
	}
	return &StreamDeltaBatcher{interval: interval, maxBytes: maxBytes, flush: flush}
}

// Feed adds a delta; may invoke flush immediately (byte threshold) or after interval.
func (d *StreamDeltaBatcher) Feed(delta string) {
	if delta == "" || d.flush == nil {
		return
	}
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.buf.WriteString(delta)
	if d.buf.Len() >= d.maxBytes {
		chunk := d.buf.String()
		d.buf.Reset()
		d.flush(chunk)
		return
	}
	d.timer = time.AfterFunc(d.interval, func() {
		if d.buf.Len() == 0 {
			return
		}
		chunk := d.buf.String()
		d.buf.Reset()
		d.flush(chunk)
	})
}

// Stop cancels the pending timer without flushing buffered text.
func (d *StreamDeltaBatcher) Stop() {
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}

// FlushRemaining drains any buffered text and stops the timer.
func (d *StreamDeltaBatcher) FlushRemaining() {
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	if d.buf.Len() == 0 {
		return
	}
	chunk := d.buf.String()
	d.buf.Reset()
	d.flush(chunk)
}
