package pkg3

import "github.com/neilotoole/go-lg/lg"

func LogDebug() {
	lg.Debugf("doing some DEBUG logging for pkg3")
}

func LogWarn() {
	lg.Debugf("doing some WARN logging for pkg3")
}
func LogError() {
	lg.Errorf("doing some ERROR logging for pkg3")
}
