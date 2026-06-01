package keyring

import (
	"context"
	"crypto/rand"
	"errors"
	"io"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
)

// crockfordAlphabet is Douglas Crockford's Base32 alphabet, lowercased.
// Excludes the visually-ambiguous characters I, L, O, U to make IDs
// safer to read aloud, transcribe, or copy by hand.
// See https://www.crockford.com/base32.html.
const crockfordAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// IDLen is the character length of an sq-generated keyring entry ID.
// At 5 bits per character, 10 characters yield ~50 bits of entropy —
// comfortably collision-free for any realistic single-user keyring
// under service name "sq". The collision check in NewID handles the
// residual case.
const IDLen = 10

// maxIDAttempts caps NewID's collision-retry loop. At 50 bits of
// entropy the expected attempt count is 1; the cap is a safety net
// for misbehaving keyring backends, not a regular code path.
const maxIDAttempts = 8

// randSource is the entropy source used by newRandomID. Defaults to
// crypto/rand.Reader. Tests in this package override it to obtain
// deterministic IDs.
var randSource = rand.Reader

// NewID returns a fresh Crockford Base32 ID guaranteed not to collide
// with an existing entry under the sq keyring service. The returned
// ID is IDLen characters long, drawn uniformly from the Crockford
// alphabet.
//
// Errors other than secret.ErrNotFound from the underlying keyring
// (locked keychain, OS unavailable, etc.) are surfaced — they prevent
// a safe mint without knowing the keyring state.
func (s *Store) NewID(ctx context.Context) (string, error) {
	for range maxIDAttempts {
		id, err := newRandomID()
		if err != nil {
			return "", err
		}
		_, err = s.Resolve(ctx, id)
		switch {
		case errors.Is(err, secret.ErrNotFound):
			return id, nil
		case err != nil:
			return "", errz.Wrapf(err, "check keyring id %q for collision", id)
		}
		// id is already taken; retry. Astronomically improbable.
	}
	return "", errz.Errorf("could not mint a unique keyring id in %d attempts", maxIDAttempts)
}

// newRandomID returns a fresh IDLen-character ID drawn uniformly from
// the Crockford alphabet. Each character is independently sampled by
// taking the low 5 bits of a fresh random byte; the distribution is
// uniform because the source is uniform.
func newRandomID() (string, error) {
	var buf [IDLen]byte
	if _, err := io.ReadFull(randSource, buf[:]); err != nil {
		return "", errz.Wrap(err, "read random bytes for keyring id")
	}
	out := make([]byte, IDLen)
	for i, b := range buf {
		out[i] = crockfordAlphabet[b&0x1f]
	}
	return string(out), nil
}
