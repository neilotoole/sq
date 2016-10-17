package pkg1

import "github.com/neilotoole/go-lg/lg"

func LogDebug() {
	lg.Debugf("doing some DEBUG logging for pkg1")
}
func LogWarn() {
	lg.Warnf("doing some WARN logging for pkg1")
}
func LogError() {
	lg.Errorf("doing some ERROR logging for pkg1")
}
