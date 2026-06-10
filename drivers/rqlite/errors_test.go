package rqlite

import (
	"crypto/tls"
	"crypto/x509"
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

	t.Run("tls signal hint preserves existing query params", func(t *testing.T) {
		src := &source.Source{
			Handle:   "@rq",
			Type:     drivertype.Rqlite,
			Location: "rqlite://host:4001?level=strong",
		}
		out := rewriteTLSSignalError(io.EOF, src)
		require.Error(t, out)
		require.Contains(t, out.Error(), "tls=true")
		require.Contains(t, out.Error(), "level=strong")
		require.NotContains(t, out.Error(), "?level=strong?tls=true",
			"must not produce malformed double-question-mark URL")
	})
}

func TestIsCertVerificationError(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"random error", errz.New("boom"), false},
		{"x509.UnknownAuthorityError", x509.UnknownAuthorityError{}, true},
		{"x509.UnknownAuthorityError wrapped", errz.Wrap(x509.UnknownAuthorityError{}, "wrap"), true},
		// x509.HostnameError is emitted by stdlib as a value, but its Error()
		// method dereferences Certificate. Using errz.Wrap on a zero-value
		// pointer fails; we test the type-match by wrapping a substring that
		// the substring-fallback would also catch.
		{
			"x509: hostname error substring",
			errz.New("x509: certificate is valid for example.com, not host"), true,
		},
		// x509.CertificateInvalidError is no longer matched by the direct
		// errors.As check (removed in Issue #3), but its Error() string begins
		// with "x509:" so the substring fallback still catches it and returns
		// true. The correct action for an expired cert is "renew it", not
		// "add ?insecure=true"; this behavior is a known limitation of the
		// substring-fallback path, which fires only when gorqlite has already
		// serialized the error to a string in production.
		{
			"x509.CertificateInvalidError caught by substring fallback",
			x509.CertificateInvalidError{Reason: x509.Expired},
			true,
		},
		{
			"tls.CertificateVerificationError",
			&tls.CertificateVerificationError{Err: errz.New("verify")}, true,
		},
		{
			"x509: substring in serialized message",
			errz.New("rqliteApiCall: x509: certificate signed by unknown authority"), true,
		},
		{
			"tls.RecordHeaderError is not a cert error",
			tls.RecordHeaderError{Msg: "bad"},
			false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isCertVerificationError(tc.err))
		})
	}
}

func TestRewriteCertVerificationError(t *testing.T) {
	src := newTestSrc(t)

	t.Run("nil passes through", func(t *testing.T) {
		require.NoError(t, rewriteCertVerificationError(nil, src))
	})

	t.Run("non-cert error passes through unchanged", func(t *testing.T) {
		in := errz.New("connection refused")
		out := rewriteCertVerificationError(in, src)
		require.Same(t, in, out)
	})

	t.Run("cert error gets the actionable hint", func(t *testing.T) {
		in := x509.UnknownAuthorityError{}
		out := rewriteCertVerificationError(in, src)
		require.Error(t, out)
		require.Contains(t, out.Error(), "certificate verification failed")
		require.Contains(t, out.Error(), "tls=true")
		require.Contains(t, out.Error(), "insecure=true")
		require.Contains(t, out.Error(), "@rq")
	})

	t.Run("cert error hint uses & when location already has ?", func(t *testing.T) {
		srcTLS := &source.Source{
			Handle:   "@rq",
			Type:     drivertype.Rqlite,
			Location: "rqlite://host:4001?tls=true",
		}
		in := x509.UnknownAuthorityError{}
		out := rewriteCertVerificationError(in, srcTLS)
		require.Error(t, out)
		// suggestLocWithParams merges via url.Values; keys sort alphabetically.
		require.Contains(t, out.Error(), "insecure=true")
		require.Contains(t, out.Error(), "tls=true")
		// Verify we did NOT produce the malformed "?tls=true?tls=true" form.
		require.NotContains(t, out.Error(), "?tls=true?tls=true")
		// Verify tls=true is not duplicated in the URL.
		require.NotContains(t, out.Error(), "tls=true&tls=true")
	})

	t.Run("cert error hint includes tls=true even when location has unrelated query", func(t *testing.T) {
		src := &source.Source{
			Handle:   "@rq",
			Type:     drivertype.Rqlite,
			Location: "rqlite://host:4001?level=strong",
		}
		in := x509.UnknownAuthorityError{}
		out := rewriteCertVerificationError(in, src)
		require.Error(t, out)
		require.Contains(t, out.Error(), "tls=true")
		require.Contains(t, out.Error(), "insecure=true")
		require.Contains(t, out.Error(), "level=strong")
	})
}

func TestEnrichConnError(t *testing.T) {
	src := newTestSrc(t)

	t.Run("nil", func(t *testing.T) {
		require.NoError(t, enrichConnError(nil, src))
	})

	t.Run("plain unwrappable error returned unchanged", func(t *testing.T) {
		in := errz.New("unrelated boom")
		out := enrichConnError(in, src)
		require.Equal(t, in.Error(), out.Error())
	})

	t.Run("tls signal wins for an io.EOF", func(t *testing.T) {
		out := enrichConnError(io.EOF, src)
		require.Contains(t, out.Error(), "appears to require TLS")
	})

	t.Run("cert error gets the cert hint", func(t *testing.T) {
		out := enrichConnError(x509.UnknownAuthorityError{}, src)
		require.Contains(t, out.Error(), "certificate verification failed")
	})
}
