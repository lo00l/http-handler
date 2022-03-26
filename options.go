package handler

import (
	"log"
	"net/http"
)

// Option is a common interface for defining options
// to change default Handler's behaviour.
type Option interface {
	apply(h *Handler)
}

type clientOption struct {
	client *http.Client
}

// WithClient creates new Option which replaces
// default HTTP client with user-provided one.
func WithClient(client *http.Client) Option {
	return &clientOption{
		client: client,
	}
}

func (opt *clientOption) apply(h *Handler) {
	h.client = opt.client
}

type loggerOption struct {
	logger *log.Logger
}

// WithLogger creates new Options which sets custom logger.
func WithLogger(logger *log.Logger) Option {
	return &loggerOption{
		logger: logger,
	}
}

func (opt *loggerOption) apply(h *Handler) {
	h.logger = opt.logger
}
