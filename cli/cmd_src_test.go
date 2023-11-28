package cli_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCmdSrc(t *testing.T) {
	ctx := context.Background()
	th := testh.New(t)
	_ = th

	tr := testrun.New(ctx, t, nil).Add()
	// err := tr.Exec("src")
	// require.NoError(t, err)

	tr.Reset().Add(*th.Source(sakila.CSVActor))
	err := tr.Exec("src")
	require.NoError(t, err)

	err = tr.Reset().Exec(".data | .[0:5]")
	require.NoError(t, err)
	t.Log(tr.Out.String())
}
