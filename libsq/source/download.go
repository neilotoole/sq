package source

import (
	"time"

	"github.com/neilotoole/sq/libsq/core/options"
)

var OptHTTPPingTimeout = options.NewDuration(
	// FIXME: apply OptHTTPPingTimeout to httpz.NewClient invocations
	"http.ping.timeout",
	"",
	0,
	time.Second*10,
	"HTTP/S ping timeout duration",
	`How long to wait for initial response from HTTP/S endpoint before
timeout occurs. Long-running operations, such as HTTP file downloads, are
not affected by this option. Example: 500ms or 3s.`,
	options.TagSource,
)

var OptHTTPSInsecureSkipVerify = options.NewBool(
	// FIXME: apply OptHTTPSInsecureSkipVerify to httpz.NewClient invocations
	"https.insecure-skip-verify",
	"",
	false,
	0,
	false,
	"Skip HTTPS TLS verification",
	"Skip HTTPS TLS verification. Useful when downloading against self-signed certs.",
)
