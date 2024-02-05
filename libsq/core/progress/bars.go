package progress

import (
	"fmt"
	"os"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/vbauerster/mpb/v8/decor"
)

// NewByteCounter returns a new determinate bar whose label
// metric is the size in bytes of the data being processed. The caller is
// ultimately responsible for calling Bar.Stop on the returned Bar.
func (p *Progress) NewByteCounter(msg string, size int64, opts ...Opt) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{msg: msg, total: size}

	var counter decor.Decorator
	var percent decor.Decorator
	if size < 0 {
		cfg.style = spinnerStyle(p.colors.Filler)
		counter = decor.Current(decor.SizeB1024(0), "% .1f")
	} else {
		cfg.style = barStyle(p.colors.Filler)
		counter = decor.Counters(decor.SizeB1024(0), "% .1f / % .1f")
		percent = decor.NewPercentage(" %.1f", decor.WCSyncSpace)
		percent = colorize(percent, p.colors.Percent)
	}
	counter = colorize(counter, p.colors.Size)
	cfg.decorators = []decor.Decorator{counter, percent}

	return p.newBar(cfg, opts)
}

// NewFilesizeCounter returns a new indeterminate bar whose label metric is a
// filesize, or "-" if it can't be read. If f is non-nil, its size is used; else
// the file at path fp is used. The caller is ultimately responsible for calling
// Bar.Stop on the returned Bar.
func (p *Progress) NewFilesizeCounter(msg string, f *os.File, fp string, opts ...Opt) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{msg: msg, total: -1, style: spinnerStyle(p.colors.Filler)}

	d := decor.Any(func(statistics decor.Statistics) string {
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

		return fmt.Sprintf("% .1f", decor.SizeB1024(fi.Size()))
	})

	cfg.decorators = []decor.Decorator{colorize(d, p.colors.Size)}
	return p.newBar(cfg, opts)
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
func (p *Progress) NewUnitCounter(msg, unit string, opts ...Opt) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{
		msg:   msg,
		total: -1,
		style: spinnerStyle(p.colors.Filler),
	}

	d := decor.Any(func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current)
		if unit != "" {
			s += " " + english.PluralWord(int(statistics.Current), unit, "")
		}
		return s
	})
	cfg.decorators = []decor.Decorator{colorize(d, p.colors.Size)}

	return p.newBar(cfg, opts)
}

// NewWaiter returns a generic indeterminate spinner. If arg clock
// is true, a timer is shown. This produces output similar to:
//
//	@excel/remote: start download                ●∙∙                4s
//
// The caller is ultimately responsible for calling Bar.Stop on the
// returned Bar.
func (p *Progress) NewWaiter(msg string, clock bool, opts ...Opt) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{
		msg:   msg,
		total: -1,
		style: spinnerStyle(p.colors.Filler),
	}

	if clock {
		d := newElapsedSeconds(p.colors.Size, time.Now(), decor.WCSyncSpace)
		cfg.decorators = []decor.Decorator{d}
	}
	return p.newBar(cfg, opts)
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
func (p *Progress) NewUnitTotalCounter(msg, unit string, total int64, opts ...Opt) *Bar {
	if p == nil {
		return nil
	}

	if total <= 0 {
		return p.NewUnitCounter(msg, unit)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{
		msg:   msg,
		total: total,
		style: barStyle(p.colors.Filler),
	}

	d := decor.Any(func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current) + " / " + humanize.Comma(statistics.Total)
		if unit != "" {
			s += " " + english.PluralWord(int(statistics.Current), unit, "")
		}
		return s
	})
	cfg.decorators = []decor.Decorator{colorize(d, p.colors.Size)}
	return p.newBar(cfg, opts)
}

// NewTimeoutWaiter returns a new indeterminate bar whose label is the
// amount of time remaining until expires. It produces output similar to:
//
//	Locking @sakila                    ●∙∙              timeout in 7s
//
// The caller is ultimately responsible for calling Bar.Stop on
// the returned bar, although the bar will also be stopped when the
// parent Progress stops.
func (p *Progress) NewTimeoutWaiter(msg string, expires time.Time, opts ...Opt) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{
		msg:   msg,
		style: spinnerStyle(p.colors.Waiting),
	}

	d := decor.Any(func(statistics decor.Statistics) string {
		remaining := time.Until(expires)
		switch {
		case remaining > 0:
			return p.colors.Size.Sprintf("timeout in %s", remaining.Round(time.Second))
		case remaining > -time.Second:
			// We do the extra second to prevent a "flash" of the timeout message,
			// and it also prevents "timeout in -1s" etc. This situation should be
			// rare; the caller should have already called Stop() on the Progress
			// when the timeout happened, but we'll play it safe.
			return p.colors.Size.Sprint("timeout in 0s")
		default:
			return p.colors.Warning.Sprintf("timed out")
		}
	})

	cfg.decorators = []decor.Decorator{d}
	cfg.total = int64(time.Until(expires))
	return p.newBar(cfg, opts)
}
