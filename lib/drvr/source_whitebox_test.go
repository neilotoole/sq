package drvr

import (
	"fmt"
	"testing"

	_ "github.com/neilotoole/sq/test/gotestutil"

	"github.com/stretchr/testify/require"
)

func TestCheckHandleValue(t *testing.T) {

	var fails = []struct {
		handle string
		msg    string
	}{
		{"", "empty is invalid"},
		{"  ", "no whitespace"},
		{"handle", "must start with @"},
		{"@", "needs at least one char"},
		{"1handle", "must start with @"},
		{"@ handle", "no whitespace"},
		{"@handle ", "no whitespace"},
		{"@handle#", "no special chars"},
		{"@1handle", "2nd char must be letter"},
		{"@1", "2nd char must be letter"},
		{"@?handle", "2nd char must be letter"},
		{"@?handle#", "no special chars"},
		{"@ha\nndle", "no newlines"},
	}

	for i, fail := range fails {
		require.Error(t, CheckHandleValue(fail.handle), fmt.Sprintf("[%d] %s]", i, fail.msg))
	}

	var passes = []string{
		"@handle",
		"@handle1",
		"@h1",
		"@h_",
		"@h__",
		"@h__1",
		"@h__1__a___",
	}

	for i, pass := range passes {
		require.Nil(t, CheckHandleValue(pass), fmt.Sprintf("[%d] should pass", i))
	}

}
