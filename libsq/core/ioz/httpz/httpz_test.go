package httpz_test

import (
	"context"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOptHeaderTimeout(t *testing.T) {
	t.Parallel()
	const (
		headerTimeout = time.Second * 2
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
			c:       httpz.NewClient2(httpz.OptHeaderTimeout(headerTimeout)),
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
					time.Sleep(time.Second)
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
