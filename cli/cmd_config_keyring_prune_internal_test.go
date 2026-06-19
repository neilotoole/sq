package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/tablew"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// fakePruner is a keyringPruner whose Delete fails for any path in failOn.
// It lets tests exercise the prune delete-failure path that the real
// go-keyring mock backend cannot trigger (its Delete always succeeds).
type fakePruner struct {
	failOn  map[string]error
	stored  []string
	deleted []string
}

func (f *fakePruner) List(context.Context) ([]string, error) {
	return f.stored, nil
}

func (f *fakePruner) Delete(_ context.Context, path string) error {
	if err := f.failOn[path]; err != nil {
		return err
	}
	f.deleted = append(f.deleted, path)
	return nil
}

// TestConfigKeyringPrune_RequiresConfigLock guards that prune takes the config
// lock, since it deletes keyring entries based on the config's references and
// must not race a concurrent config writer.
func TestConfigKeyringPrune_RequiresConfigLock(t *testing.T) {
	require.True(t, cmdRequiresConfigLock(newConfigKeyringPruneCmd()))
}

// TestPruneOrphans_DeleteFailure verifies that when one entry's deletion
// fails, prune still deletes the others, marks the failed row, and returns
// a summary error naming the failure count.
func TestPruneOrphans_DeleteFailure(t *testing.T) {
	kr := &fakePruner{
		stored: []string{"keepgoing9", "boom123456"},
		failOn: map[string]error{"boom123456": errz.New("keyring locked")},
	}
	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := tablew.NewKeyringWriter(buf, pr)

	// nil sources means every stored entry is an orphan.
	err := pruneOrphans(context.Background(), kr, nil, false, w)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete 1 of 2 orphaned keyring entries")
	require.Equal(t, []string{"keepgoing9"}, kr.deleted,
		"the non-failing orphan must still be deleted after the failure")

	out := buf.String()
	require.Contains(t, out, "boom123456")
	require.Contains(t, out, "failed")
}
