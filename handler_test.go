package handler

import (
	"bytes"
	"fmt"
	"github.com/r3labs/diff/v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSemaphore(t *testing.T) {
	s := newSemaphore(20)

	for i := 0; i < 20; i++ {
		if !s.acquire() {
			t.Fatal("failed to acquire semaphore")
		}
	}

	if s.acquire() {
		t.Fatal("semaphore has been acquired but should not have")
	}

	for i := 0; i < 5; i++ {
		s.release()
	}

	for i := 0; i < 5; i++ {
		if !s.acquire() {
			t.Fatal("failed to acquire semaphore")
		}
	}

	for i := 0; i < 20; i++ {
		s.release()
	}
}

// TestHandlerOneIncomingRequest checks case when requests limit is not exceeded.
func TestHandlerRequestLimitNotExceeded(t *testing.T) {
	requestsLimit := 5
	clientTimeout := time.Millisecond * 500

	server := createServer(clientTimeout)

	s := httptest.NewServer(NewHandler(WithClient(server.Client()), LimitRequests(requestsLimit)))

	var wg sync.WaitGroup
	ch := make(chan error)

	for i := 0; i < requestsLimit; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			resp, err := s.Client().Post(
				s.URL,
				"text/plain",
				getRequestBodyBuffer(
					getUrl(server.URL, 100, time.Millisecond*100), // should be in time
					getUrl(server.URL, 200, time.Millisecond*600), // should not be in time
					getUrl(server.URL, 300, time.Millisecond*300), // should be in time
					getUrl(server.URL, 400, time.Millisecond*50),  // should be in time
					getUrl(server.URL, 500, time.Millisecond*550), // should not be in time
				),
			)
			if err != nil {
				ch <- fmt.Errorf("failed to make request: %s", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				ch <- fmt.Errorf("bad status code: %d", resp.StatusCode)
			}

			if err := checkResponse(resp, []int{100, 300, 400}); err != nil {
				ch <- err
			}
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for err := range ch {
		t.Error(err)
	}
}

// TestHandlerOneIncomingRequest checks case when requests limit is exceeded.
func TestHandlerRequestLimitExceeded(t *testing.T) {
	requestsLimit := 5
	clientTimeout := time.Millisecond * 500

	server := createServer(clientTimeout)

	s := httptest.NewServer(NewHandler(WithClient(server.Client()), LimitRequests(requestsLimit)))

	var exceededRequests uint64

	var wg sync.WaitGroup
	errCh := make(chan error)
	exceedCh := make(chan struct{})

	for i := 0; i < requestsLimit*2; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			resp, err := s.Client().Post(
				s.URL,
				"text/plain",
				getRequestBodyBuffer(
					getUrl(server.URL, 100, time.Millisecond*100), // should be in time
					getUrl(server.URL, 200, time.Millisecond*600), // should not be in time
					getUrl(server.URL, 300, time.Millisecond*300), // should be in time
					getUrl(server.URL, 400, time.Millisecond*50),  // should be in time
					getUrl(server.URL, 500, time.Millisecond*550), // should not be in time
				),
			)
			if err != nil {
				errCh <- fmt.Errorf("failed to make request: %s", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusServiceUnavailable {
				exceedCh <- struct{}{}

				return
			}

			if resp.StatusCode != http.StatusOK {
				errCh <- fmt.Errorf("bad status code: %d", resp.StatusCode)
			}

			if err := checkResponse(resp, []int{100, 300, 400}); err != nil {
				errCh <- err
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errCh)
		close(exceedCh)
	}()

	go func() {
		for range exceedCh {
			exceededRequests++
		}
	}()

	for err := range errCh {
		t.Error(err)
	}

	if exceededRequests != 5 {
		log.Fatalf("wrong number of exceeded requests, expected %d, got %d", 5, exceededRequests)
	}
}

func TestHandler_ServeHTTP(t *testing.T) {
	server := createServer(0)

	http.Handle("/", NewHandler(WithClient(server.Client())))

	go func() {
		http.ListenAndServe("127.0.0.1:8082", nil)
	}()

	var wg sync.WaitGroup

	for i := 0; i < 120; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			resp, err := http.Post(
				"http://127.0.0.1:8082",
				"text/plain",
				bytes.NewBufferString(getUrl(server.URL, 100, time.Second)),
			)
			if err != nil {
				fmt.Print(err)

				return
			}
			defer resp.Body.Close()

			c, err := ioutil.ReadAll(resp.Body)

			fmt.Println(string(c))
		}()
	}

	wg.Wait()
}

func createServer(clientTimeout time.Duration) *httptest.Server {
	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		length, _ := strconv.ParseUint(request.URL.Query().Get("length"), 10, 64)
		timeout, _ := time.ParseDuration(request.URL.Query().Get("timeout"))

		time.Sleep(timeout)

		writer.Write(bytes.Repeat([]byte{' '}, int(length)))
	}))
	s.Client().Timeout = clientTimeout

	return s
}

func getUrl(base string, responseLength int, responseTimeout time.Duration) string {
	q := make(url.Values, 2)
	q.Set("length", strconv.FormatInt(int64(responseLength), 10))
	q.Set("timeout", responseTimeout.String())

	u, _ := url.Parse(base)
	u.RawQuery = q.Encode()

	return u.String()
}

func getRequestBodyBuffer(urls ...string) io.Reader {
	return bytes.NewBufferString(strings.Join(urls, "\n"))
}

func checkResponse(resp *http.Response, expectedData []int) error {
	responseData := make([]int, 0)

	for {
		var v int
		if _, err := fmt.Fscanf(resp.Body, "%d", &v); err != nil {
			break
		}

		responseData = append(responseData, v)
	}

	if d, _ := diff.Diff(responseData, expectedData); len(d) != 0 {
		return fmt.Errorf("expected data does not match response data: %s", d)
	}

	return nil
}
