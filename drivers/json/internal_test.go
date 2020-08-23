package json

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/sakila"
)

func TestPredictColKindsJSONA(t *testing.T) {
	f, err := os.Open("testdata/actor.jsona")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })

	kinds, err := predictColKindsJSONA(context.Background(), f)
	require.NoError(t, err)
	require.Equal(t, sakila.TblActorColKinds(), kinds)
}
