package sqlz

import (
	"context"
	"database/sql/driver"
	"io"
)

// ConnectorWith returns a stdlib driver.Connector that wraps the
// supplied impl, applying the non-nil fn to the connection returned
// by Connect. If fn is nil, impl is returned unchanged.
//
// If impl implements io.Closer, the returned connector will also
// implement io.Closer. This behavior conforms to the expectations
// of driver.Connector:
//
//	If a Connector implements io.Closer, the sql package's DB.Close
//	method will call Close and return error (if any).
func ConnectorWith(impl driver.Connector, fn func(ctx context.Context, conn driver.Conn) error) driver.Connector {
	if fn == nil {
		return impl
	}

	c := connector{impl: impl, fn: fn}

	if _, ok := impl.(io.Closer); ok {
		return connectorCloser{c}
	}
	return c
}

var _ driver.Connector = (*connector)(nil)

type connector struct {
	impl driver.Connector
	fn   func(ctx context.Context, conn driver.Conn) error
}

// Connect implements driver.Connector.
func (c connector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.impl.Connect(ctx)
	if err != nil {
		return nil, err
	}

	if err = c.fn(ctx, conn); err != nil {
		return nil, err
	}

	return conn, nil
}

// Driver implements driver.Connector.
func (c connector) Driver() driver.Driver {
	return c.impl.Driver()
}

var (
	_ driver.Connector = (*connectorCloser)(nil)
	_ io.Closer        = (*connectorCloser)(nil)
)

type connectorCloser struct {
	connector
}

// Close implements io.Closer.
func (c connectorCloser) Close() error {
	return c.impl.(io.Closer).Close()
}
