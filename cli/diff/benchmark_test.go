package diff_test

import (
	"bytes"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"testing"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/testh/sakila"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/diff"
	"github.com/neilotoole/sq/cli/output/tablew"
	"github.com/neilotoole/sq/testh"
)

func BenchmarkExecTableDiff(b *testing.B) {
	th := testh.New(b, testh.OptNoLog())

	ru := th.Run()
	require.NoError(b, cli.FinishRunInit(th.Context, ru))
	srcA := testh.NewSakilaSource(b, "@a", false)
	srcB := testh.NewSakilaSource(b, "@b", false)
	require.NoError(b, ru.Config.Collection.Add(srcA))
	require.NoError(b, ru.Config.Collection.Add(srcB))

	elems := &diff.Elements{Data: true}
	cfg := &diff.Config{
		RecordWriterFn: tablew.NewRecordWriter,
		Lines:          3,
		Printing:       output.NewPrinting(),
		Colors:         diffdoc.NewColors(),
		Concurrency:    tuning.OptErrgroupLimit.Get(options.FromContext(th.Context)),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := &bytes.Buffer{}
		ru.Out = buf
		err := diff.ExecTableDiff(
			th.Context,
			ru,
			cfg,
			elems,
			srcA.Handle,
			sakila.TblActor,
			srcB.Handle,
			sakila.TblActor,
		)
		require.NoError(b, err)
		buf.Reset()
	}
}
