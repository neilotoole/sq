package sqlz_test

import (
	"context"
	"database/sql/driver"
	"io"
	"testing"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
)

var _ driver.Connector = (*tConnector)(nil)

type tConnector struct{}

func (t tConnector) Connect(_ context.Context) (driver.Conn, error) {
	return &sqlite3.SQLiteConn{}, nil
}

func (t tConnector) Driver() driver.Driver {
	return &sqlite3.SQLiteDriver{}
}

var (
	_ driver.Connector = (*tConnectorCloser)(nil)
	_ io.Closer        = (*tConnectorCloser)(nil)
)

type tConnectorCloser struct{}

func (t tConnectorCloser) Connect(_ context.Context) (driver.Conn, error) {
	return &sqlite3.SQLiteConn{}, nil
}

func (t tConnectorCloser) Driver() driver.Driver {
	return &sqlite3.SQLiteDriver{}
}

func (t tConnectorCloser) Close() error {
	return nil
}

func TestConnectorWith(t *testing.T) {
	var c driver.Connector
	var invoked bool
	fn := func(ctx context.Context, conn driver.Conn) error {
		invoked = true
		return nil
	}

	// Test that ConnectorWith returns the same connector
	// if fn is nil.
	c = tConnector{}
	c2 := sqlz.ConnectorWith(c, nil)
	require.Equal(t, c, c2)

	c = tConnector{}
	c2 = sqlz.ConnectorWith(c, fn)
	require.NotEqual(t, c, c2)

	_, ok := c2.(io.Closer)
	require.False(t, ok, "shouldn't be an io.Closer")

	conn, err := c2.Connect(context.Background())
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.True(t, invoked)
	invoked = false // reset

	// Test that ConnectorWith returns a connector that
	// implements io.Closer if the underlying connector
	// implements io.Closer.
	c = tConnectorCloser{}
	c2 = sqlz.ConnectorWith(c, fn)
	_, ok = c2.(io.Closer)
	require.True(t, ok, "should be an io.Closer")

	conn, err = c2.Connect(context.Background())
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.True(t, invoked)
}
