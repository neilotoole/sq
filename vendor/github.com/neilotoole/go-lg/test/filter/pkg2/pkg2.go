package pkg2

import "github.com/neilotoole/go-lg/lg"

func LogDebug() {
	lg.Debugf("doing some DEBUG logging for pkg2")
}

func LogWarn() {
	lg.Debugf("doing some WARN logging for pkg2")
}

func LogError() {
	lg.Errorf("doing some ERROR logging for pkg2")
}
