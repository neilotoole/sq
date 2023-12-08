package progress

import (
	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	// NewByteCounter returns a new progress bar whose metric is the count
	// of bytes processed. If the size is unknown, set arg size to -1. The caller
	// is ultimately responsible for calling [Bar.Stop] on the returned Bar.
	// However, the returned Bar is also added to the Progress's cleanup list,
	// so it will be called automatically when the Progress is shut down, but that
	// may be later than the actual conclusion of the Bar's work.
)

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
// responsible for calling [Bar.Stop] on the returned Bar. However,
// the returned Bar is also added to the Progress's cleanup list, so
// it will be called automatically when the Progress is shut down, but that
// may be later than the actual conclusion of the spinner's work.
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

// NewUnitTotalCounter returns a new determinate bar whose label
// metric is the plural of the provided unit. The caller is ultimately
// responsible for calling [Bar.Stop] on the returned Bar. However,
// the returned Bar is also added to the Progress's cleanup list, so
// it will be called automatically when the Progress is shut down, but that
// may be later than the actual conclusion of the Bar's work.
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
