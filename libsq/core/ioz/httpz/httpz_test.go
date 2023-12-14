package httpz_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/testh/tu"
)

func TestOptRequestTimeout(t *testing.T) {
	t.Parallel()
	const srvrBody = `Hello World!`
	serverDelay := time.Millisecond * 200
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			t.Log("Server request context done")
			return
		case <-time.After(serverDelay):
		}
		_, err := w.Write([]byte(srvrBody))
		assert.NoError(t, err)
	}))
	t.Cleanup(srvr.Close)

	clientRequestTimeout := time.Millisecond * 100
	c := httpz.NewClient(httpz.OptRequestTimeout(clientRequestTimeout))
	req, err := http.NewRequest(http.MethodGet, srvr.URL, nil)
	require.NoError(t, err)

	resp, err := c.Do(req)
	t.Log(err)
	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "http request not completed within")
	require.True(t, errors.Is(err, context.DeadlineExceeded))
}

// TestOptHeaderTimeout_correct_error verifies that an HTTP request
// that fails via OptHeaderTimeout returns the correct error.
func TestOptHeaderTimeout_correct_error(t *testing.T) {
	t.Parallel()
	const srvrBody = `Hello World!`
	serverDelay := time.Second * 2
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			t.Log("Server request context done")
			return
		case <-time.After(serverDelay):
		}
		_, err := w.Write([]byte(srvrBody))
		assert.NoError(t, err)
	}))
	t.Cleanup(srvr.Close)

	clientHeaderTimeout := time.Second * 1
	c := httpz.NewClient(httpz.OptHeaderTimeout(clientHeaderTimeout))
	req, err := http.NewRequest(http.MethodGet, srvr.URL, nil)
	require.NoError(t, err)

	resp, err := c.Do(req)
	t.Log(err)
	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "http response not received within")
	require.True(t, errors.Is(err, context.DeadlineExceeded))

	// Now let's try again, with a shorter server delay, so the
	// request should succeed.
	serverDelay = time.Millisecond
	resp, err = c.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got := tu.ReadToString(t, resp.Body)
	require.Equal(t, srvrBody, got)
}

// TestOptHeaderTimeout_vs_stdlib verifies that OptHeaderTimeout
// works as expected when compared to stdlib.
func TestOptHeaderTimeout_vs_stdlib(t *testing.T) {
	t.Parallel()
	const (
		headerTimeout = time.Millisecond * 200
		numLines      = 7
	)

	testCases := []struct {
		name    string
		ctxFn   func(t *testing.T) context.Context
		c       *http.Client
		wantErr bool
	}{
		{
			name: "http.DefaultClient",
			ctxFn: func(t *testing.T) context.Context {
				ctx, cancelFn := context.WithTimeout(context.Background(), headerTimeout)
				t.Cleanup(cancelFn)
				return ctx
			},
			c:       http.DefaultClient,
			wantErr: true,
		},
		{
			name: "headerTimeout",
			ctxFn: func(t *testing.T) context.Context {
				return context.Background()
			},
			c:       httpz.NewClient(httpz.OptHeaderTimeout(headerTimeout)),
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for i := 0; i < numLines; i++ {
					select {
					case <-r.Context().Done():
						t.Logf("Server exiting due to: %v", r.Context().Err())
						return
					default:
					}
					if _, err := io.WriteString(w, string(rune('A'+i))+"\n"); err != nil {
						t.Logf("Server write err: %v", err)
						return
					}
					w.(http.Flusher).Flush()
					time.Sleep(time.Millisecond * 100)
				}
			}))
			t.Cleanup(slowServer.Close)

			ctx := tc.ctxFn(t)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, slowServer.URL, nil)
			require.NoError(t, err)

			resp, err := tc.c.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			// Sleep long enough to trigger the header timeout.
			time.Sleep(headerTimeout + time.Second)
			b, err := io.ReadAll(resp.Body)
			if tc.wantErr {
				require.Error(t, err)
				t.Logf("err: %T: %v", err, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, b, numLines*2) // *2 because of the newlines.
		})
	}
}
