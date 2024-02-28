package progress

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/vbauerster/mpb/v8"

	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/vbauerster/mpb/v8/decor"
)

// BarOpt is a functional option for Bar creation.
type BarOpt interface {
	apply(*Progress, *barConfig)
}

// barConfig is passed to Progress.createBar. Note that there are four decorator
// fields: these are effectively the "widgets" that are displayed on any given
// bar. If a widget is nil, a nopWidget will be set by createVirtualBar. This is
// because we need the widgets to exist (even if invisible) for visual
// alignment purposes.
type barConfig struct {
	style         mpb.BarFillerBuilder
	counterWidget decor.Decorator
	percentWidget decor.Decorator
	timerWidget   decor.Decorator
	memoryWidget  decor.Decorator
	msgWidget     decor.Decorator
	total         int64
}

// createBar creates a new bar, and adds it to Progress.allBars.
//
// The caller must hold Progress.mu.
func (p *Progress) createBar(cfg *barConfig, opts []BarOpt) Bar {
	if p == nil {
		return nopBar{}
	}

	// FIXME: createBar should probably acquire the lock internally.

	vb := newVirtualBar(p, cfg, opts)
	p.allBars = append(p.allBars, vb)
	return vb
}

// forgetBar removes bar vb from Progress.allBars. It is the caller's
// responsibility to first invoke virtualBar.destroy.
func (p *Progress) forgetBar(vb *virtualBar) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.allBars = langz.Remove(p.allBars, vb)
}

// NewByteCounter returns a new determinate bar whose label metric is the size
// in bytes of the data being processed. The caller is ultimately responsible
// for calling Bar.Stop on the returned Bar.
func (p *Progress) NewByteCounter(msg string, size int64, opts ...BarOpt) Bar {
	if p == nil {
		return nopBar{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{
		msgWidget: staticMsgWidget(p, msg),
		total:     size,
	}

	if size < 0 {
		cfg.style = spinnerStyle(p.colors.Filler)
		cfg.counterWidget = colorize(
			decor.Current(decor.SizeB1000(0), "% .1f", p.align.counter),
			p.colors.Size,
		)
	} else {
		cfg.style = barStyle(p.colors.Filler)
		cfg.counterWidget = colorize(
			decor.Counters(decor.SizeB1000(0), "% .1f / % .1f", p.align.counter),
			p.colors.Size,
		)
		cfg.percentWidget = colorize(
			decor.NewPercentage(" %.1f", p.align.percent),
			p.colors.Percent,
		)
	}

	return p.createBar(cfg, opts)
}

// NewFilesizeCounter returns a new indeterminate bar whose label metric is a
// filesize, or "-" if it can't be read. If f is non-nil, its size is used; else
// the file at path fp is used. The caller is ultimately responsible for calling
// Bar.Stop on the returned Bar.
func (p *Progress) NewFilesizeCounter(msg string, f *os.File, fp string, opts ...BarOpt) Bar {
	if p == nil {
		return nopBar{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{
		msgWidget: staticMsgWidget(p, msg),
		total:     -1,
		style:     spinnerStyle(p.colors.Filler),
	}

	fn := func(statistics decor.Statistics) string {
		var fi os.FileInfo
		var err error
		if f != nil {
			fi, err = f.Stat()
		} else {
			fi, err = os.Stat(fp)
		}

		if err != nil {
			return "-"
		}

		return fmt.Sprintf("% .1f", decor.SizeB1000(fi.Size()))
	}

	cfg.counterWidget = colorize(decor.Any(fn, p.align.counter), p.colors.Size)

	return p.createBar(cfg, opts)
}

// NewUnitCounter returns a new indeterminate bar whose label
// metric is the plural of the provided unit. The caller is ultimately
// responsible for calling Bar.Stop on the returned Bar.
//
//	bar := p.NewUnitCounter("Ingest records", "rec")
//	defer bar.Stop()
//
//	for i := 0; i < 100; i++ {
//	    bar.Incr(1)
//	    time.Sleep(100 * time.Millisecond)
//	}
//
// This produces output similar to:
//
//	Ingesting records               ∙∙●              87 recs
//
// Note that the unit arg is automatically pluralized.
func (p *Progress) NewUnitCounter(msg, unit string, opts ...BarOpt) Bar {
	if p == nil {
		return nopBar{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{
		msgWidget: staticMsgWidget(p, msg),
		total:     -1,
		style:     spinnerStyle(p.colors.Filler),
	}

	fn := func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current)
		if unit != "" {
			s += " " + english.PluralWord(int(statistics.Current), unit, "")
		}
		return s
	}

	cfg.counterWidget = colorize(decor.Any(fn, p.align.counter), p.colors.Size)

	return p.createBar(cfg, opts)
}

// NewWaiter returns a generic indeterminate spinner with a timer. This produces
// output similar to:
//
//	@excel/remote: start download                ●∙∙                4s
//
// The caller is ultimately responsible for calling Bar.Stop on the
// returned Bar.
func (p *Progress) NewWaiter(msg string, opts ...BarOpt) Bar {
	if p == nil {
		return nopBar{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	opts = append(opts, OptTimer)

	cfg := &barConfig{
		msgWidget: staticMsgWidget(p, msg),
		total:     -1,
		style:     spinnerStyle(p.colors.Filler),
	}

	return p.createBar(cfg, opts)
}

// NewUnitTotalCounter returns a new determinate bar whose label
// metric is the plural of the provided unit. The caller is ultimately
// responsible for calling Bar.Stop on the returned Bar.
//
// This produces output similar to:
//
//	Ingesting sheets   ∙∙∙∙∙●                     4 / 16 sheets
//
// Note that the unit arg is automatically pluralized.
func (p *Progress) NewUnitTotalCounter(msg, unit string, total int64, opts ...BarOpt) Bar {
	if p == nil {
		return nopBar{}
	}

	if total <= 0 {
		return p.NewUnitCounter(msg, unit)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{
		msgWidget: staticMsgWidget(p, msg),
		total:     total,
		style:     barStyle(p.colors.Filler),
	}

	fn := func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current) + " / " + humanize.Comma(statistics.Total)
		if unit != "" {
			s += " " + english.PluralWord(int(statistics.Current), unit, "")
		}
		return s
	}

	cfg.counterWidget = colorize(decor.Any(fn, p.align.counter), p.colors.Size)
	return p.createBar(cfg, opts)
}

// NewTimeoutWaiter returns a new indeterminate bar whose label is the
// amount of time remaining until expires. It produces output similar to:
//
//	Locking @sakila                    ●∙∙              timeout in 7s
//
// The caller is ultimately responsible for calling Bar.Stop on
// the returned bar, although the bar will also be stopped when the
// parent Progress stops.
func (p *Progress) NewTimeoutWaiter(msg string, expires time.Time, opts ...BarOpt) Bar {
	if p == nil {
		return nopBar{}
	}

	cfg := &barConfig{
		msgWidget: staticMsgWidget(p, msg),
		style:     spinnerStyle(p.colors.Waiting),
	}

	fn := func(statistics decor.Statistics) string {
		remaining := time.Until(expires)
		switch {
		case remaining > 0:
			return fmt.Sprintf("timeout in %s", remaining.Round(time.Second))
		case remaining > -time.Second:
			// We do the extra second to prevent a "flash" of the timeout message,
			// and it also prevents "timeout in -1s" etc. This situation should be
			// rare; the caller should have already called Stop() on the Progress
			// when the timeout happened, but we'll play it safe.
			return "timeout in 0s"
		default:
			return "timed out"
		}
	}

	cfg.counterWidget = decor.Meta(decor.Any(fn, p.align.counter), func(s string) string {
		if strings.HasPrefix(s, "timeout in") {
			return p.colors.Size.Sprint(s)
		}
		return p.colors.Warning.Sprint(s)
	})

	cfg.total = int64(time.Until(expires))

	p.mu.Lock()
	defer p.mu.Unlock()

	return p.createBar(cfg, opts)
}
