package fscache

import (
	"os"
	"time"
)

// FileInfo is just a wrapper around os.FileInfo which includes atime.
type FileInfo struct {
	os.FileInfo
	Atime time.Time
}

type fileInfo struct {
	name     string
	size     int64
	fileMode os.FileMode
	isDir    bool
	sys      interface{}
	wt       time.Time
}

func (f *fileInfo) Name() string {
	return f.name
}

func (f *fileInfo) Size() int64 {
	return f.size
}

func (f *fileInfo) Mode() os.FileMode {
	return f.fileMode
}

func (f *fileInfo) ModTime() time.Time {
	return f.wt
}

func (f *fileInfo) IsDir() bool {
	return f.isDir
}

func (f *fileInfo) Sys() interface{} {
	return f.sys
}

// AccessTime returns the last time the file was read.
// It will be used to check expiry of a file, and must be concurrent safe
// with modifications to the FileSystem (writes, reads etc.)
func (f *FileInfo) AccessTime() time.Time {
	return f.Atime
}
