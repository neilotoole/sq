package progress

import (
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// NewByteCounter returns a new determinate bar whose label
// metric is the size in bytes of the data being processed. The caller is
// ultimately responsible for calling [Bar.Stop] on the returned Bar.
func (p *Progress) NewByteCounter(msg string, size int64) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var style mpb.BarFillerBuilder
	var counter decor.Decorator
	var percent decor.Decorator
	if size < 0 {
		style = spinnerStyle(p.colors.Filler)
		counter = decor.Current(decor.SizeB1024(0), "% .1f")
	} else {
		style = barStyle(p.colors.Filler)
		counter = decor.Counters(decor.SizeB1024(0), "% .1f / % .1f")
		percent = decor.NewPercentage(" %.1f", decor.WCSyncSpace)
		percent = colorize(percent, p.colors.Percent)
	}
	counter = colorize(counter, p.colors.Size)

	return p.newBar(msg, size, style, counter, percent)
}

// NewUnitCounter returns a new indeterminate bar whose label
// metric is the plural of the provided unit. The caller is ultimately
// responsible for calling [Bar.Stop] on the returned Bar.
//
//	bar := p.NewUnitCounter("Ingest records", "rec")
//	defer bar.Stop()
//
//	for i := 0; i < 100; i++ {
//	    bar.IncrBy(1)
//	    time.Sleep(100 * time.Millisecond)
//	}
//
// This produces output similar to:
//
//	Ingesting records               ∙∙●              87 recs
//
// Note that the unit arg is automatically pluralized.
func (p *Progress) NewUnitCounter(msg, unit string) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	decorator := decor.Any(func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current)
		if unit != "" {
			s += " " + english.PluralWord(int(statistics.Current), unit, "")
		}
		return s
	})
	decorator = colorize(decorator, p.colors.Size)

	style := spinnerStyle(p.colors.Filler)

	return p.newBar(msg, -1, style, decorator)
}

// NewWaiter returns a generic indeterminate spinner. If arg clock
// is true, a timer is shown. This produces output similar to:
//
//	@excel/remote: start download                ●∙∙                4s
//
// The caller is ultimately responsible for calling [Bar.Stop] on the
// returned Bar.
func (p *Progress) NewWaiter(msg string, clock bool) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var d []decor.Decorator
	if clock {
		d = append(d, newElapsedSeconds(p.colors.Size, time.Now(), decor.WCSyncSpace))
	}
	style := spinnerStyle(p.colors.Filler)
	return p.newBar(msg, -1, style, d...)
}

// NewUnitTotalCounter returns a new determinate bar whose label
// metric is the plural of the provided unit. The caller is ultimately
// responsible for calling [Bar.Stop] on the returned Bar.
//
// This produces output similar to:
//
//	Ingesting sheets   ∙∙∙∙∙●                     4 / 16 sheets
//
// Note that the unit arg is automatically pluralized.
func (p *Progress) NewUnitTotalCounter(msg, unit string, total int64) *Bar {
	if p == nil {
		return nil
	}

	if total <= 0 {
		return p.NewUnitCounter(msg, unit)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	style := barStyle(p.colors.Filler)
	decorator := decor.Any(func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current) + " / " + humanize.Comma(statistics.Total)
		if unit != "" {
			s += " " + english.PluralWord(int(statistics.Current), unit, "")
		}
		return s
	})
	decorator = colorize(decorator, p.colors.Size)
	return p.newBar(msg, total, style, decorator)
}

// NewTimeoutWaiter returns a new indeterminate bar whose label is the
// amount of time remaining until expires. It produces output similar to:
//
//	Locking @sakila                    ●∙∙              timeout in 7s
//
// The caller is ultimately responsible for calling [Bar.Stop] on
// the returned bar, although the bar will also be stopped when the
// parent Progress stops.
func (p *Progress) NewTimeoutWaiter(msg string, expires time.Time) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	style := spinnerStyle(p.colors.Waiting)
	decorator := decor.Any(func(statistics decor.Statistics) string {
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

	total := time.Until(expires)
	return p.newBar(msg, int64(total), style, decorator)
}
