package rqlite

import (
	"crypto/tls"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func newTestSrc(t *testing.T) *source.Source {
	t.Helper()
	return &source.Source{
		Handle:   "@rq",
		Type:     drivertype.Rqlite,
		Location: "rqlite://host:4001",
	}
}

func TestIsTLSSignal(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"random error", errz.New("boom"), false},
		{"tls record header error", tls.RecordHeaderError{Msg: "x"}, true},
		{"tls record header error wrapped", errz.Wrap(tls.RecordHeaderError{Msg: "x"}, "wrap"), true},
		{"io.EOF direct", io.EOF, true},
		{"io.EOF wrapped", errz.Wrap(io.EOF, "wrap"), true},
		{
			"canonical 400 body substring",
			errz.New("Bad Request: Client sent an HTTP request to an HTTPS server"), true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isTLSSignal(tc.err))
		})
	}
}

func TestRewriteTLSSignalError(t *testing.T) {
	src := newTestSrc(t)

	t.Run("nil passes through", func(t *testing.T) {
		require.NoError(t, rewriteTLSSignalError(nil, src))
	})

	t.Run("non-tls error passes through unchanged", func(t *testing.T) {
		in := errz.New("dns lookup failed")
		out := rewriteTLSSignalError(in, src)
		// Verify the exact same error pointer is returned, not just an equal
		// message. A re-wrap would produce a distinct *errz with a different
		// stack, so the pointers would differ.
		require.Same(t, in, out)
	})

	t.Run("tls signal error gets the actionable hint", func(t *testing.T) {
		in := io.EOF
		out := rewriteTLSSignalError(in, src)
		require.Error(t, out)
		require.Contains(t, out.Error(), "appears to require TLS")
		require.Contains(t, out.Error(), "?tls=true")
		require.Contains(t, out.Error(), "&insecure=true")
		require.Contains(t, out.Error(), "@rq")
	})
}
