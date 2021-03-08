package libsq_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSLQ2SQL(t *testing.T) {
	testCases := []struct {
		handles []string
		slq     string
		wantSQL string
		wantErr bool
	}{
		{
			handles: []string{sakila.SL3},
			slq:     `@sakila_sl3 | .actor, .film_actor | join(.film_actor.actor_id == .actor.actor_id)`,
			wantSQL: `SELECT * FROM "actor" INNER JOIN "film_actor" ON "film_actor"."actor_id" = "actor"."actor_id"`,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(testh.Name(tc.slq), func(t *testing.T) {
			th := testh.New(t)
			srcs := th.NewSourceSet(tc.handles...)

			gotSQL, gotErr := libsq.EngineSLQ2SQL(th.Context, th.Log, th.Databases(), th.Databases(), srcs, tc.slq)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)

			require.Equal(t, tc.wantSQL, gotSQL)
		})
	}

}
