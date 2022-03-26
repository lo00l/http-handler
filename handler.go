// Package handler defined Handler type which implements http.Handler interface.

// Handler expects to receive POST requests only.
// Request body should contain list of URL, each URL on separate line.
// Once POST request is received, Handler reads its content, splits it into lines, and fetches URLs.
// Response consists of fetched documents' lengths, separated by new line. Result set is not guaranteed to be sorted.
// All errors (non 2XX response codes, timeouts, etc) are logged.

// While creating Handler, additional options can be provided to change its default behaviour.
// See: WithClient, WithLogger.

package handler

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
)

const defaultMaxIncomingRequests = 100

var defaultLogger = log.Default()
var defaultClient = http.DefaultClient

// semaphore is used to limit number
// of concurrent incoming requests.
type semaphore struct {
	ch chan struct{}
}

// newSemaphore creates new semaphore.
func newSemaphore(cap int) *semaphore {
	return &semaphore{
		ch: make(chan struct{}, cap),
	}
}

// semaphore tries to increase semaphore counter
// and returns true on success, and false otherwise.
func (s *semaphore) acquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// release decreases semaphore counter.
func (s *semaphore) release() {
	<-s.ch
}

type Handler struct {
	sem    *semaphore
	logger *log.Logger
	client *http.Client
}

// NewHandler created Handler and applies provided options.
func NewHandler(opts ...Option) *Handler {
	h := &Handler{
		sem: newSemaphore(defaultMaxIncomingRequests),
	}

	for _, opt := range opts {
		opt.apply(h)
	}

	if h.client == nil {
		h.client = defaultClient
	}
	if h.logger == nil {
		h.logger = defaultLogger
	}

	return h
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)

		return
	}

	if !h.sem.acquire() {
		http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)

		return
	}
	defer h.sem.release()

	data, err := ioutil.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return
	}

	urls := strings.Split(string(data), "\n")

	writer.Header().Add("Content-Type", "text/plain")

	for size := range h.fetch(urls) {
		fmt.Fprintln(writer, size)
	}
}

// fetch concurrently fetches provided URLs.
// It returns channel received documents' lengths is sent to.
// After all documents are fetched, then channel is cloed.
func (h *Handler) fetch(urls []string) <-chan int {
	ch := make(chan int)

	go func() {
		var wg sync.WaitGroup

		for _, url := range urls {
			wg.Add(1)

			go func(url string) {
				defer wg.Done()

				resp, err := h.client.Get(url)
				if err != nil {
					h.logger.Println(err)

					return
				}

				content, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					h.logger.Println(err)

					return
				}

				ch <- len(content)
			}(url)
		}

		wg.Wait()

		close(ch)
	}()

	return ch
}
