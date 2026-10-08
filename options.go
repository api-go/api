package api

import (
	"context"
	"time"

	"apigo.cc/go/log"
)

// CallOptions controls one API invocation without changing the stored Action.
type CallOptions struct {
	Context context.Context
	Token   string
	Config  map[string]any
	Timeout time.Duration
	Logger  *log.Logger
	Stream  *StreamOptions
	Timing  bool
	// InputPreview logs at most 20 Unicode characters from the first text input.
	InputPreview bool
}

// StreamOptions receives the upstream response synchronously on the calling goroutine.
type StreamOptions struct {
	OnHeaders func(result map[string]any) error
	OnData    func(data []byte) error
	OnDone    func(result map[string]any) error
}

func firstOptions(options []*CallOptions) *CallOptions {
	if len(options) > 0 && options[0] != nil {
		return options[0]
	}
	return &CallOptions{}
}
