package fscache

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"
)

func Example() {
	// create the cache, keys expire after 1 hour.
	c, err := New("./cache", 0755, time.Hour)
	if err != nil {
		log.Fatal(err.Error())
	}

	// wipe the cache when done
	defer c.Clean()

	// Get() and it's streams can be called concurrently but just for example:
	for i := 0; i < 3; i++ {
		r, w, err := c.Get("stream")
		if err != nil {
			log.Fatal(err.Error())
		}

		if w != nil { // a new stream, write to it.
			go func() {
				w.Write([]byte("hello world\n"))
				w.Close()
			}()
		}

		// the stream has started, read from it
		io.Copy(os.Stdout, r)
		r.Close()
	}
	// Output:
	// hello world
	// hello world
	// hello world
}

func ExampleHandler() {
	c, err := New("./server", 0700, 0)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer c.Clean()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello Client")
	})

	ts := httptest.NewServer(Handler(c, handler))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		log.Fatal(err.Error())
	}
	io.Copy(os.Stdout, resp.Body)
	resp.Body.Close()
	// Output:
	// Hello Client
}
