package source_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// addSrc is a helper that adds a source with the given handle to coll,
// failing the test on error.
func addSrc(t *testing.T, coll *source.Collection, handle string) {
	t.Helper()
	require.NoError(t, coll.Add(newSource(handle)))
}

// TestCollection_Data exercises Collection.Data, including the nil
// receiver guard.
func TestCollection_Data(t *testing.T) {
	var nilColl *source.Collection
	require.Nil(t, nilColl.Data())

	coll := &source.Collection{}
	addSrc(t, coll, "@src1")
	require.NotNil(t, coll.Data())
}

// TestCollection_Add_errPaths covers the error branches of Add.
func TestCollection_Add_errPaths(t *testing.T) {
	coll := &source.Collection{}

	// Invalid handle.
	require.Error(t, coll.Add(&source.Source{Handle: "no-at-prefix"}))

	// Valid add.
	addSrc(t, coll, "@src1")

	// Duplicate handle conflict.
	err := coll.Add(newSource("@src1"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

// TestCollection_renameSource_errPaths covers error branches of
// renameSource via RenameSource.
func TestCollection_renameSource_errPaths(t *testing.T) {
	t.Run("invalid_new_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.RenameSource("@src1", "bad-handle")
		require.Error(t, err)
	})

	t.Run("old_handle_not_found", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.RenameSource("@nope", "@src2")
		require.Error(t, err)
	})

	t.Run("noop_same_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		got, err := coll.RenameSource("@src1", "@src1")
		require.NoError(t, err)
		require.Equal(t, "@src1", got.Handle)
	})

	t.Run("new_handle_exists", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		addSrc(t, coll, "@src2")
		_, err := coll.RenameSource("@src1", "@src2")
		require.Error(t, err)
		require.Contains(t, err.Error(), "already exists")
	})

	t.Run("new_handle_conflicts_with_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		addSrc(t, coll, "@src1")
		// @prod is an existing group; renaming @src1 to @prod must fail.
		_, err := coll.RenameSource("@src1", "@prod")
		require.Error(t, err)
		require.Contains(t, err.Error(), "existing group")
	})

	t.Run("new_handle_nests_under_existing_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod")
		addSrc(t, coll, "@src1")
		// @prod is an existing handle; @prod/db nests below it.
		_, err := coll.RenameSource("@src1", "@prod/db")
		require.Error(t, err)
		require.Contains(t, err.Error(), "existing handle")
	})

	t.Run("active_src_follows_rename", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.SetActive("@src1", false)
		require.NoError(t, err)

		_, err = coll.RenameSource("@src1", "@src2")
		require.NoError(t, err)
		require.Equal(t, "@src2", coll.ActiveHandle())
	})

	t.Run("active_group_reassigned_when_emptied", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		require.NoError(t, coll.SetActiveGroup("prod"))

		// Move the only source out of prod; prod no longer exists, so
		// the active group must be reassigned to the source's new group.
		_, err := coll.RenameSource("@prod/db", "@staging/db")
		require.NoError(t, err)
		require.Equal(t, "staging", coll.ActiveGroup())
	})
}

// TestCollection_RenameGroup_errPaths covers RenameGroup error and edge
// branches.
func TestCollection_RenameGroup_errPaths(t *testing.T) {
	t.Run("cannot_rename_root", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.RenameGroup("/", "prod")
		require.Error(t, err)
		_, err = coll.RenameGroup("", "prod")
		require.Error(t, err)
	})

	t.Run("invalid_old_group", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.RenameGroup("@bad", "prod")
		require.Error(t, err)
	})

	t.Run("invalid_new_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		_, err := coll.RenameGroup("prod", "@bad")
		require.Error(t, err)
	})

	t.Run("old_group_not_exist", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.RenameGroup("nope", "prod")
		require.Error(t, err)
	})

	t.Run("new_group_nests_under_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod")
		addSrc(t, coll, "@staging/db")
		_, err := coll.RenameGroup("staging", "prod/sub")
		require.Error(t, err)
		require.Contains(t, err.Error(), "existing handle")
	})

	t.Run("rename_collides_with_existing_target", func(t *testing.T) {
		// Renaming group "a" to "b" tries to rename @a/x -> @b/x, but
		// @b/x already exists, so renameSource fails inside the loop.
		coll := &source.Collection{}
		addSrc(t, coll, "@a/x")
		addSrc(t, coll, "@b/x")
		_, err := coll.RenameGroup("a", "b")
		require.Error(t, err)
	})

	t.Run("rename_to_root_via_slash", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		require.NoError(t, coll.SetActiveGroup("prod"))

		srcs, err := coll.RenameGroup("prod", "/")
		require.NoError(t, err)
		require.Len(t, srcs, 1)
		require.Equal(t, "@db", srcs[0].Handle)
		require.Equal(t, "/", coll.ActiveGroup())
	})

	t.Run("active_group_follows_rename", func(t *testing.T) {
		// Active group "prod" holds no direct source members, only a
		// subgroup. The inner renameSource never reassigns the active
		// group (no src's own group equals "prod"), so RenameGroup's
		// own active-group update handles it.
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/sub/db")
		require.NoError(t, coll.SetActiveGroup("prod"))

		srcs, err := coll.RenameGroup("prod", "production")
		require.NoError(t, err)
		require.Len(t, srcs, 1)
		require.Equal(t, "production", coll.ActiveGroup())
	})
}

// TestCollection_MoveHandleToGroup_errPaths covers MoveHandleToGroup
// error and switch branches.
func TestCollection_MoveHandleToGroup_errPaths(t *testing.T) {
	t.Run("handle_not_found", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.MoveHandleToGroup("@nope", "prod")
		require.Error(t, err)
	})

	t.Run("invalid_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.MoveHandleToGroup("@src1", "@bad")
		require.Error(t, err)
	})

	t.Run("group_nests_under_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod")
		addSrc(t, coll, "@src1")
		_, err := coll.MoveHandleToGroup("@src1", "prod/sub")
		require.Error(t, err)
	})

	t.Run("move_root_src_to_group", func(t *testing.T) {
		// oldGroup == "" branch.
		coll := &source.Collection{}
		addSrc(t, coll, "@db")
		got, err := coll.MoveHandleToGroup("@db", "prod")
		require.NoError(t, err)
		require.Equal(t, "@prod/db", got.Handle)
	})

	t.Run("move_grouped_src_to_root", func(t *testing.T) {
		// toGroup == "/" branch.
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		got, err := coll.MoveHandleToGroup("@prod/db", "/")
		require.NoError(t, err)
		require.Equal(t, "@db", got.Handle)
	})

	t.Run("move_grouped_src_to_other_group", func(t *testing.T) {
		// default switch branch.
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		got, err := coll.MoveHandleToGroup("@prod/db", "staging")
		require.NoError(t, err)
		require.Equal(t, "@staging/db", got.Handle)
	})
}

// TestCollection_ActiveHandle_empty covers the no-active-source branch.
func TestCollection_ActiveHandle_empty(t *testing.T) {
	coll := &source.Collection{}
	require.Empty(t, coll.ActiveHandle())

	addSrc(t, coll, "@src1")
	_, err := coll.SetActive("@src1", false)
	require.NoError(t, err)
	require.Equal(t, "@src1", coll.ActiveHandle())
}

// TestCollection_active_staleActiveSrc covers active() when ActiveSrc
// points at a non-existent handle.
func TestCollection_active_staleActiveSrc(t *testing.T) {
	coll := &source.Collection{}
	addSrc(t, coll, "@src1")
	// Force-set an active src that doesn't exist.
	_, err := coll.SetActive("@ghost", true)
	require.NoError(t, err)

	// active() should return nil because @ghost isn't in sources.
	require.Nil(t, coll.Active())
}

// TestCollection_Scratch_branches covers Scratch including the
// stale-handle branch.
func TestCollection_Scratch_branches(t *testing.T) {
	coll := &source.Collection{}
	// No scratch set.
	require.Nil(t, coll.Scratch())

	addSrc(t, coll, "@src1")
	_, err := coll.SetScratch("@src1")
	require.NoError(t, err)
	require.NotNil(t, coll.Scratch())

	// Remove the source; scratch handle becomes stale.
	require.NoError(t, coll.Remove("@src1"))
	require.Nil(t, coll.Scratch())

	// A scratch handle pointing at a non-existent source (e.g. from a
	// hand-edited or stale config) yields nil.
	stale := &source.Collection{}
	err = stale.UnmarshalJSON([]byte(`{"scratch":"@ghost","sources":[` +
		`{"handle":"@real","driver":"sqlite3","location":"/tmp/a.db"}]}`))
	require.NoError(t, err)
	require.Nil(t, stale.Scratch())
}

// TestCollection_get_branches covers get's special cases.
func TestCollection_get_branches(t *testing.T) {
	coll := &source.Collection{}
	addSrc(t, coll, "@src1")

	t.Run("empty_handle", func(t *testing.T) {
		_, err := coll.Get("   ")
		require.Error(t, err)
	})

	t.Run("missing_at_prefix_added", func(t *testing.T) {
		got, err := coll.Get("src1")
		require.NoError(t, err)
		require.Equal(t, "@src1", got.Handle)
	})

	t.Run("active_handle_no_active", func(t *testing.T) {
		_, err := coll.Get(source.ActiveHandle)
		require.Error(t, err)
	})

	t.Run("active_handle_with_active", func(t *testing.T) {
		_, err := coll.SetActive("@src1", false)
		require.NoError(t, err)
		got, err := coll.Get(source.ActiveHandle)
		require.NoError(t, err)
		require.Equal(t, "@src1", got.Handle)
	})

	t.Run("unknown_handle", func(t *testing.T) {
		_, err := coll.Get("@nope")
		require.Error(t, err)
	})
}

// TestCollection_setActive_branches covers setActive paths.
func TestCollection_setActive_branches(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.SetActive("@src1", false)
		require.NoError(t, err)

		got, err := coll.SetActive("", false)
		require.NoError(t, err)
		require.Nil(t, got)
		require.Empty(t, coll.ActiveHandle())
	})

	t.Run("invalid_handle", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.SetActive("bad-handle", false)
		require.Error(t, err)
	})

	t.Run("force_nonexistent", func(t *testing.T) {
		coll := &source.Collection{}
		got, err := coll.SetActive("@ghost", true)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("force_existent", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		got, err := coll.SetActive("@src1", true)
		require.NoError(t, err)
		require.NotNil(t, got)
	})

	t.Run("not_force_nonexistent", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.SetActive("@ghost", false)
		require.Error(t, err)
	})
}

// TestCollection_SetScratch_branches covers SetScratch paths.
func TestCollection_SetScratch_branches(t *testing.T) {
	coll := &source.Collection{}
	addSrc(t, coll, "@src1")

	// Unset.
	got, err := coll.SetScratch("")
	require.NoError(t, err)
	require.Nil(t, got)

	// Unknown.
	_, err = coll.SetScratch("@ghost")
	require.Error(t, err)

	// Valid.
	got, err = coll.SetScratch("@src1")
	require.NoError(t, err)
	require.NotNil(t, got)
}

// TestCollection_RemoveGroup_branches covers RemoveGroup paths.
func TestCollection_RemoveGroup_branches(t *testing.T) {
	t.Run("unknown_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		_, err := coll.RemoveGroup("nope")
		require.Error(t, err)
	})

	t.Run("removes_and_resets_active_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db1")
		addSrc(t, coll, "@prod/db2")
		require.NoError(t, coll.SetActiveGroup("prod"))

		srcs, err := coll.RemoveGroup("prod")
		require.NoError(t, err)
		require.Len(t, srcs, 2)
		require.Equal(t, "/", coll.ActiveGroup())
	})

	t.Run("removes_keeps_active_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db1")
		addSrc(t, coll, "@staging/db2")
		require.NoError(t, coll.SetActiveGroup("staging"))

		srcs, err := coll.RemoveGroup("prod")
		require.NoError(t, err)
		require.Len(t, srcs, 1)
		require.Equal(t, "staging", coll.ActiveGroup())
	})
}

// TestCollection_remove_branches covers remove paths.
func TestCollection_remove_branches(t *testing.T) {
	t.Run("empty_collection", func(t *testing.T) {
		coll := &source.Collection{}
		require.Error(t, coll.Remove("@nope"))
	})

	t.Run("unknown_handle", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		require.Error(t, coll.Remove("@nope"))
	})

	t.Run("remove_only_source", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		require.NoError(t, coll.Remove("@src1"))
		require.Empty(t, coll.Handles())
	})

	t.Run("remove_active_source", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@src1")
		addSrc(t, coll, "@src2")
		_, err := coll.SetActive("@src1", false)
		require.NoError(t, err)
		require.NoError(t, coll.Remove("@src1"))
		require.Empty(t, coll.ActiveHandle())
	})

	t.Run("remove_resets_active_group", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		addSrc(t, coll, "@other")
		require.NoError(t, coll.SetActiveGroup("prod"))
		// Removing the only source in prod removes the group; active
		// group must reset to root.
		require.NoError(t, coll.Remove("@prod/db"))
		require.Equal(t, "/", coll.ActiveGroup())
	})

	t.Run("remove_middle_source", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@a")
		addSrc(t, coll, "@b")
		addSrc(t, coll, "@c")
		require.NoError(t, coll.Remove("@b"))
		require.ElementsMatch(t, []string{"@a", "@c"}, coll.Handles())
	})
}

// TestCollection_handlesInGroup_branches covers handlesInGroup.
func TestCollection_handlesInGroup_branches(t *testing.T) {
	coll := &source.Collection{}
	addSrc(t, coll, "@prod/db1")
	addSrc(t, coll, "@prod/db2")
	addSrc(t, coll, "@other")

	// Root group returns all handles.
	all, err := coll.HandlesInGroup("/")
	require.NoError(t, err)
	require.Len(t, all, 3)

	// Specific group.
	got, err := coll.HandlesInGroup("prod")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"@prod/db1", "@prod/db2"}, got)

	// Unknown group.
	_, err = coll.HandlesInGroup("nope")
	require.Error(t, err)
}

// TestCollection_Clone_nil covers Clone's nil-receiver guard.
func TestCollection_Clone_nil(t *testing.T) {
	var coll *source.Collection
	require.Nil(t, coll.Clone())
}

// TestCollection_Tree_branches covers Tree's nil guard and error paths.
func TestCollection_Tree_branches(t *testing.T) {
	t.Run("nil_receiver", func(t *testing.T) {
		var coll *source.Collection
		got, err := coll.Tree("")
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("default_fromGroup", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/db")
		got, err := coll.Tree("")
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, source.RootGroup, got.Name)
	})

	t.Run("invalid_group", func(t *testing.T) {
		coll := &source.Collection{}
		_, err := coll.Tree("@bad")
		require.Error(t, err)
	})

	t.Run("nested_tree", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/sub/db")
		addSrc(t, coll, "@prod/db2")
		got, err := coll.Tree("prod")
		require.NoError(t, err)
		require.Equal(t, "prod", got.Name)
		require.NotEmpty(t, got.Groups)
	})
}

// TestVerifyIntegrity_branches covers the harder VerifyIntegrity paths.
func TestVerifyIntegrity_branches(t *testing.T) {
	t.Run("invalid_source_in_collection", func(t *testing.T) {
		coll := &source.Collection{}
		// Add a valid source, then corrupt it via marshaling roundtrip
		// is awkward; instead build via JSON to bypass Add validation.
		err := coll.UnmarshalJSON([]byte(`{"sources":[{"handle":"@bad","driver":"","location":""}]}`))
		require.NoError(t, err)
		repaired, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.False(t, repaired)
	})

	t.Run("nil_source_in_collection", func(t *testing.T) {
		coll := &source.Collection{}
		err := coll.UnmarshalJSON([]byte(`{"sources":[null]}`))
		require.NoError(t, err)
		repaired, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.False(t, repaired)
	})

	t.Run("duplicate_handle", func(t *testing.T) {
		coll := &source.Collection{}
		err := coll.UnmarshalJSON([]byte(`{"sources":[` +
			`{"handle":"@dup","driver":"sqlite3","location":"/tmp/a.db"},` +
			`{"handle":"@dup","driver":"sqlite3","location":"/tmp/b.db"}]}`))
		require.NoError(t, err)
		repaired, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.False(t, repaired)
	})

	t.Run("stale_active_source_repaired", func(t *testing.T) {
		coll := &source.Collection{}
		err := coll.UnmarshalJSON([]byte(`{"active.source":"@ghost","sources":[` +
			`{"handle":"@real","driver":"sqlite3","location":"/tmp/a.db"}]}`))
		require.NoError(t, err)
		repaired, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.True(t, repaired)
		require.Empty(t, coll.ActiveHandle())
	})
}

// TestSource_LogValue_nilAndExtras covers nil receiver, Catalog and
// Schema attribute branches.
func TestSource_LogValue_nilAndExtras(t *testing.T) {
	log := lgt.New(t)

	var nilSrc *source.Source
	require.Equal(t, slog.KindAny, nilSrc.LogValue().Kind())

	src := &source.Source{
		Handle:   "@sakila",
		Type:     drivertype.Pg,
		Location: "postgres://user:pass@localhost/sakila",
		Catalog:  "cat",
		Schema:   "sch",
	}
	log.Debug("src with catalog and schema", lga.Src, src)
}

// TestValidSource_viaVerifyIntegrity exercises validSource error
// branches (empty location, unknown driver type) through the public
// VerifyIntegrity path.
func TestValidSource_viaVerifyIntegrity(t *testing.T) {
	t.Run("empty_location", func(t *testing.T) {
		coll := &source.Collection{}
		err := coll.UnmarshalJSON([]byte(`{"sources":[{"handle":"@x","driver":"sqlite3","location":""}]}`))
		require.NoError(t, err)
		_, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.Contains(t, verr.Error(), "location is empty")
	})

	t.Run("unknown_driver", func(t *testing.T) {
		coll := &source.Collection{}
		err := coll.UnmarshalJSON([]byte(`{"sources":[{"handle":"@x","driver":"","location":"/tmp/x.db"}]}`))
		require.NoError(t, err)
		_, verr := source.VerifyIntegrity(coll)
		require.Error(t, verr)
		require.Contains(t, verr.Error(), "driver type")
	})
}

// TestContains_extra covers the empty-slice fast path and the
// *Source typed path.
func TestContains_extra(t *testing.T) {
	require.False(t, source.Contains[string](nil, "@x"))

	s1 := newSource("@a")
	s2 := newSource("@b")
	srcs := []*source.Source{s1, s2}

	require.True(t, source.Contains(srcs, s1))
	require.False(t, source.Contains(srcs, newSource("@a")))
	require.True(t, source.Contains(srcs, "@b"))
	require.False(t, source.Contains(srcs, "@z"))
}

// TestParseTableHandle_extra covers the remaining ParseTableHandle
// branches (multi-period input).
func TestParseTableHandle_extra(t *testing.T) {
	// More than one period -> invalid.
	_, _, err := source.ParseTableHandle("@h.a.b")
	require.Error(t, err)
}

// TestSuggestHandle_extra covers SuggestHandle branches not hit by the
// existing tests: location parse error, empty-name fallback to "h", and
// the active-group prefix.
func TestSuggestHandle_extra(t *testing.T) {
	t.Run("parse_error", func(t *testing.T) {
		coll := &source.Collection{}
		// A bare scheme with no driver and an unparseable location.
		_, err := source.SuggestHandle(coll, drivertype.None, "://://bad")
		require.Error(t, err)
	})

	t.Run("active_group_prefix", func(t *testing.T) {
		coll := &source.Collection{}
		addSrc(t, coll, "@prod/seed")
		require.NoError(t, coll.SetActiveGroup("prod"))
		got, err := source.SuggestHandle(coll, drivertype.SQLite, "/path/to/actor.csv")
		require.NoError(t, err)
		require.Equal(t, "@prod/actor", got)
	})

	t.Run("empty_name_becomes_h", func(t *testing.T) {
		coll := &source.Collection{}
		// A SQL Server DSN with no database yields an empty parsed
		// Name, so finalizeSuggestedHandle falls back to "h".
		got, err := source.SuggestHandle(coll, drivertype.MSSQL, "sqlserver://user:pass@localhost")
		require.NoError(t, err)
		require.Equal(t, "@h", got)
	})
}

// TestSort_bothNil covers the a==nil && b==nil branch of Sort and
// SortGroups (multiple nil elements must be compared against each other).
func TestSort_bothNil(t *testing.T) {
	srcs := []*source.Source{nil, newSource("@b"), nil, newSource("@a"), nil}
	source.Sort(srcs)
	require.Nil(t, srcs[0])
	require.Nil(t, srcs[1])
	require.Nil(t, srcs[2])
	require.Equal(t, "@a", srcs[3].Handle)
	require.Equal(t, "@b", srcs[4].Handle)

	groups := []*source.Group{nil, {Name: "b"}, nil, {Name: "a"}, nil}
	source.SortGroups(groups)
	require.Nil(t, groups[0])
	require.Nil(t, groups[1])
	require.Nil(t, groups[2])
	require.Equal(t, "a", groups[3].Name)
	require.Equal(t, "b", groups[4].Name)
}
