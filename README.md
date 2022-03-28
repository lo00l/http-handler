# HTTP handler

Golang package providing HTTP handler which receives POST request with body containing list of URLs separated with new line, tries to fetch provided URLs concurrently, and then returns response with fetched documents' lengths separated with new line. All unsuccessful outgoing requests (wrong URL format, timeout, etc) are logged.

## Installation
```shell
go get github.com/lo00l/http-handler
```

## Usage
Create new handler by calling `NewHandler()` and then register a HTTP server:
```go
package main

import (
	"github.com/lo00l/http-handler"
	"log"
	"net/http"
)

func main() {
	h := handler.NewHandler()

	http.Handle("/", h)

	if err := http.ListenAndServe("127.0.0.1:8000", nil); err != nil {
		log.Fatal(err)
	}
}
```

Suppose content of `urls.txt` file looks like this:
```text
https://google.com
https://twitter.com
https://fb.com
```

Then after running abode program and executing `curl -X POST --data-binary "@urls.txt" http://127.0.0.1:8000` the output should be something like this:
```text
17195
96432
89327
```

Note that response items are not guaranteed to be sorted.

### Customize

It's also possible to pass some options to `NewHandler()` function to change default handler's behaviour.

`WithClient()` option sets HTTP client which will be used to make outgoing requests. By default, `http.DefaultClient` is used.
```go
// create client with timeout and use it in Handler
client := &http.Client{
		Timeout: time.Second * 5,
	}
h := handler.NewHandler(handler.WithClient(client))
```

`WithLogger()` option set logger which will be used to log unsuccessful requests. By default, `log.Default()` is used.
```go
f, err := os.Create("handler.log")
if err != nil {
    log.Fatal(err)
}
defer f.Close()

logger := log.New(f, "", log.LstdFlags)
```

`LimitRequests()` limits number of concurrent incoming requests. By default, limit is 100.
```go
h := handler.NewHandler(handler.LimitRequests(20))
```

It's possible to pass any number of options:
```go
h := handler.NewHandler(opt1, opt2, opt3)
```