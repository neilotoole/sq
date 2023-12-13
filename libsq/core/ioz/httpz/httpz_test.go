package httpz_test

import (
	"context"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestNewHTTPClient_headerTimeout(t *testing.T) {
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
			c:       httpz.NewClient("", false, headerTimeout, 0),
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

func TestTimeout1(t *testing.T) {
	const urlPaymentLargeCSV = "https://sqio-public.s3.amazonaws.com/testdata/payment-large.gen.csv"

	const urlActorCSV = "https://sq.io/testdata/actor.csv"
	const respTimeout = time.Second * 2
	const lines = 10
	const wantLen = lines * 2
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < lines; i++ {
			select {
			case <-r.Context().Done():
				t.Logf("Server exiting due to: %v", r.Context().Err())
				return
			default:
			}
			_, _ = io.WriteString(w, string(rune('A'+i))+"\n")
			w.(http.Flusher).Flush()
			time.Sleep(time.Second)
		}
	}))
	t.Cleanup(slowServer.Close)

	ctx, cancelFn := context.WithTimeout(context.Background(), respTimeout)
	defer cancelFn()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, slowServer.URL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// cancelFn()
	time.Sleep(time.Second * 3)

	select {
	case <-ctx.Done():
		t.Logf("ctx is done: %v", ctx.Err())
	default:
		t.Logf("ctx is not done")
		cancelFn()
	}

	// cancelFn()
	b, err := io.ReadAll(resp.Body)
	t.Logf("err: %T: %v", err, err)
	t.Logf("len(b): %d", len(b))
	t.Logf("b:\n\n%s\n\n", b)
	assert.Error(t, err)
	// require.Nil(t, b)
	_ = b
	// require.Len(t, b, 0)
	// require.Len(t, b, 7641)
}
