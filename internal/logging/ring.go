package logging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Entry is one captured log record, flattened into the text the console shows.
type Entry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// Ring keeps the most recent records in memory so the admin console can show a
// live tail without reading the rotating file back.
type Ring struct {
	mu      sync.Mutex
	entries []Entry
	next    int
	filled  bool
}

// ringSize is how many records the process keeps: enough for a few minutes of
// ordinary play, small enough to stay irrelevant in memory.
const ringSize = 5000

var recent = &Ring{entries: make([]Entry, ringSize)}

// Recent returns up to limit records, newest last. A non-empty contains keeps
// only the records whose text holds it (case-insensitive), which is how the
// console filters by [Room] / [Relay] and friends.
func Recent(limit int, contains string) []Entry {
	recent.mu.Lock()
	defer recent.mu.Unlock()
	if limit <= 0 || limit > ringSize {
		limit = ringSize
	}
	ordered := recent.orderedLocked()
	needle := strings.ToLower(contains)
	if needle != "" {
		filtered := ordered[:0]
		for _, entry := range ordered {
			if strings.Contains(strings.ToLower(entry.Message), needle) {
				filtered = append(filtered, entry)
			}
		}
		ordered = filtered
	}
	if len(ordered) > limit {
		ordered = ordered[len(ordered)-limit:]
	}
	return ordered
}

// orderedLocked returns the records oldest first.
func (r *Ring) orderedLocked() []Entry {
	if !r.filled {
		if r.next == 0 {
			// An empty ring must still answer an empty array: a nil slice encodes as
			// null, and the console's log pane expects a list.
			return []Entry{}
		}
		return append([]Entry(nil), r.entries[:r.next]...)
	}
	out := make([]Entry, 0, len(r.entries))
	out = append(out, r.entries[r.next:]...)
	out = append(out, r.entries[:r.next]...)
	return out
}

func (r *Ring) add(entry Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[r.next] = entry
	r.next = (r.next + 1) % len(r.entries)
	if r.next == 0 {
		r.filled = true
	}
}

// ringHandler tees every record into the ring before the normal handler writes it.
type ringHandler struct {
	inner slog.Handler
	attrs []slog.Attr
}

// Capture wraps a handler so its records also land in the in-memory ring.
func Capture(inner slog.Handler) slog.Handler {
	return &ringHandler{inner: inner}
}

func (h *ringHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ringHandler) Handle(ctx context.Context, record slog.Record) error {
	message := record.Message
	appendAttr := func(attr slog.Attr) bool {
		message += " " + attr.Key + "=" + attr.Value.String()
		return true
	}
	for _, attr := range h.attrs {
		appendAttr(attr)
	}
	record.Attrs(appendAttr)
	recent.add(Entry{Time: record.Time, Level: levelName(record.Level), Message: message})
	return h.inner.Handle(ctx, record)
}

func (h *ringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ringHandler{inner: h.inner.WithAttrs(attrs), attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...)}
}

func (h *ringHandler) WithGroup(name string) slog.Handler {
	return &ringHandler{inner: h.inner.WithGroup(name), attrs: h.attrs}
}

func levelName(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARN"
	case level >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

// String renders one entry for the console's text view.
func (e Entry) String() string {
	return fmt.Sprintf("%s %-5s %s", e.Time.Format("15:04:05"), e.Level, e.Message)
}
