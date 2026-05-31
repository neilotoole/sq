package keyring

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/libsq/core/secret"
)

// withRandSource swaps randSource for the duration of t. Required
// because tests in this package mutate package-level state.
func withRandSource(t *testing.T, r io.Reader) {
	t.Helper()
	orig := randSource
	t.Cleanup(func() { randSource = orig })
	randSource = r
}

func TestCrockfordAlphabet(t *testing.T) {
	require.Len(t, crockfordAlphabet, 32, "Crockford Base32 has 32 symbols")
	// The four excluded letters must not appear.
	for _, c := range "ilou" {
		require.NotContains(t, crockfordAlphabet, string(c),
			"Crockford alphabet must exclude %q", string(c))
	}
	// Every expected character must appear.
	for _, c := range "0123456789abcdefghjkmnpqrstvwxyz" {
		require.Contains(t, crockfordAlphabet, string(c),
			"Crockford alphabet missing %q", string(c))
	}
}

func TestNewRandomID_Format(t *testing.T) {
	for range 100 {
		id, err := newRandomID()
		require.NoError(t, err)
		require.Len(t, id, IDLen, "id length must equal IDLen")
		for _, c := range id {
			require.Contains(t, crockfordAlphabet, string(c),
				"id contains non-Crockford character: %q", string(c))
		}
	}
}

func TestNewRandomID_NoNearTermDuplicates(t *testing.T) {
	// At ~50 bits of entropy, 1000 draws have collision probability
	// well below 1 in 10^9 — a duplicate here means the generator is
	// broken (e.g. constant output), not unlucky.
	seen := make(map[string]bool, 1000)
	for range 1000 {
		id, err := newRandomID()
		require.NoError(t, err)
		require.False(t, seen[id], "duplicate id in 1000 draws: %q", id)
		seen[id] = true
	}
}

func TestNewID_FreshUnderEmptyKeyring(t *testing.T) {
	gokeyring.MockInit()
	r := New()
	ctx := context.Background()

	id, err := r.NewID(ctx)
	require.NoError(t, err)
	require.Len(t, id, IDLen)

	// NewID does not write to the keyring; callers do. So the
	// returned ID still resolves to ErrNotFound.
	_, err = r.Resolve(ctx, id)
	require.ErrorIs(t, err, secret.ErrNotFound)
}

func TestNewID_NeverReturnsExisting(t *testing.T) {
	gokeyring.MockInit()
	r := New()
	ctx := context.Background()

	const occupied = "j2k7m3pxtz"
	require.NoError(t, r.Set(ctx, occupied, "value"))

	for range 100 {
		id, err := r.NewID(ctx)
		require.NoError(t, err)
		require.NotEqual(t, occupied, id)
	}
}

func TestNewID_RetriesOnCollision(t *testing.T) {
	gokeyring.MockInit()
	r := New()
	ctx := context.Background()

	// crockfordAlphabet[0]='0', so a random byte with low 5 bits = 0
	// produces a '0' character. Two IDs of all-'0' and all-'1':
	const occupied = "0000000000"
	const free = "1111111111"
	require.NoError(t, r.Set(ctx, occupied, "x"))

	// First IDLen bytes → all '0's → collides; next IDLen bytes →
	// all '1's → free. Verifies the retry path executes.
	src := bytes.NewReader(append(
		bytes.Repeat([]byte{0x00}, IDLen),
		bytes.Repeat([]byte{0x01}, IDLen)...,
	))
	withRandSource(t, src)

	id, err := r.NewID(ctx)
	require.NoError(t, err)
	require.Equal(t, free, id)
}

func TestNewID_FailsAfterMaxAttempts(t *testing.T) {
	gokeyring.MockInit()
	r := New()
	ctx := context.Background()

	const occupied = "0000000000"
	require.NoError(t, r.Set(ctx, occupied, "x"))

	// Every attempt sees zero bytes and so generates the occupied ID.
	withRandSource(t, zeroReader{})

	_, err := r.NewID(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not mint")
}

// zeroReader returns an infinite stream of zero bytes — newRandomID
// against this source always produces the all-'0' ID.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
