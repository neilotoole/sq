package fscache

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/djherbis/atime"
	"github.com/djherbis/stream"
)

// FileSystemStater implementers can provide FileInfo data about a named resource.
type FileSystemStater interface {
	// Stat takes a File.Name() and returns FileInfo interface
	Stat(name string) (FileInfo, error)
}

// FileSystem is used as the source for a Cache.
type FileSystem interface {
	// Stream FileSystem
	stream.FileSystem

	FileSystemStater

	// Reload should look through the FileSystem and call the supplied fn
	// with the key/filename pairs that are found.
	Reload(func(key, name string)) error

	// RemoveAll should empty the FileSystem of all files.
	RemoveAll() error
}

// StandardFS is an implemenation of FileSystem which writes to the os Filesystem.
type StandardFS struct {
	root string
	init func() error

	// EncodeKey takes a 'name' given to Create and converts it into a
	// the Filename that should be used. It should return 'true' if
	// DecodeKey can convert the returned string back to the original 'name'
	// and false otherwise.
	// This must be set before the first call to Create.
	EncodeKey func(string) (string, bool)

	// DecodeKey should convert a given Filename into the original 'name' given to
	// EncodeKey, and return true if this conversion was possible. Returning false
	// will cause it to try and lookup a stored 'encodedName.key' file which holds
	// the original name.
	DecodeKey func(string) (string, bool)
}

// IdentityCodeKey works as both an EncodeKey and a DecodeKey func, which just returns
// it's given argument and true. This is expected to be used when your FSCache
// uses SetKeyMapper to ensure its internal km(key) value is already a valid filename path.
func IdentityCodeKey(key string) (string, bool) { return key, true }

// NewFs returns a FileSystem rooted at directory dir.
// Dir is created with perms if it doesn't exist.
// This also uses the default EncodeKey/DecodeKey functions B64ORMD5HashEncodeKey/B64DecodeKey.
func NewFs(dir string, mode os.FileMode) (*StandardFS, error) {
	fs := &StandardFS{
		root: dir,
		init: func() error {
			return os.MkdirAll(dir, mode)
		},
		EncodeKey: B64OrMD5HashEncodeKey,
		DecodeKey: B64DecodeKey,
	}
	return fs, fs.init()
}

// Reload looks through the dir given to NewFs and returns every key, name pair (Create(key) => name = File.Name())
// that is managed by this FileSystem.
func (fs *StandardFS) Reload(add func(key, name string)) error {
	files, err := ioutil.ReadDir(fs.root)
	if err != nil {
		return err
	}

	addfiles := make(map[string]struct {
		os.FileInfo
		key string
	})

	for _, f := range files {

		if strings.HasSuffix(f.Name(), ".key") {
			continue
		}

		key, err := fs.getKey(f.Name())
		if err != nil {
			fs.Remove(filepath.Join(fs.root, f.Name()))
			continue
		}
		fi, ok := addfiles[key]

		if !ok || fi.ModTime().Before(f.ModTime()) {
			if ok {
				fs.Remove(fi.Name())
			}
			addfiles[key] = struct {
				os.FileInfo
				key string
			}{
				FileInfo: f,
				key:      key,
			}
		} else {
			fs.Remove(f.Name())
		}

	}

	for _, f := range addfiles {
		path, err := filepath.Abs(filepath.Join(fs.root, f.Name()))
		if err != nil {
			return err
		}
		add(f.key, path)
	}

	return nil
}

// Create creates a File for the given 'name', it may not use the given name on the
// os filesystem, that depends on the implementation of EncodeKey used.
func (fs *StandardFS) Create(name string) (stream.File, error) {
	name, err := fs.makeName(name)
	if err != nil {
		return nil, err
	}
	return fs.create(name)
}

func (fs *StandardFS) create(name string) (stream.File, error) {
	return os.OpenFile(filepath.Join(fs.root, name), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
}

// Open opens a stream.File for the given File.Name() returned by Create().
func (fs *StandardFS) Open(name string) (stream.File, error) {
	return os.Open(name)
}

// Remove removes a stream.File for the given File.Name() returned by Create().
func (fs *StandardFS) Remove(name string) error {
	os.Remove(fmt.Sprintf("%s.key", name))
	return os.Remove(name)
}

// RemoveAll deletes all files in the directory managed by this StandardFS.
// Warning that if you put files in this directory that were not created by
// StandardFS they will also be deleted.
func (fs *StandardFS) RemoveAll() error {
	if err := os.RemoveAll(fs.root); err != nil {
		return err
	}
	return fs.init()
}

// AccessTimes returns atime and mtime for the given File.Name() returned by Create().
func (fs *StandardFS) AccessTimes(name string) (rt, wt time.Time, err error) {
	fi, err := os.Stat(name)
	if err != nil {
		return rt, wt, err
	}
	return atime.Get(fi), fi.ModTime(), nil
}

// Stat returns FileInfo for the given File.Name() returned by Create().
func (fs *StandardFS) Stat(name string) (FileInfo, error) {
	stat, err := os.Stat(name)
	if err != nil {
		return FileInfo{}, err
	}

	return FileInfo{FileInfo: stat, Atime: atime.Get(stat)}, nil
}

const (
	saltSize    = 8
	salt        = "xxxxxxxx" // this is only important for sizing now.
	maxShort    = 20
	shortPrefix = "s"
	longPrefix  = "l"
)

func tob64(s string) string {
	buf := bytes.NewBufferString("")
	enc := base64.NewEncoder(base64.URLEncoding, buf)
	enc.Write([]byte(s))
	enc.Close()
	return buf.String()
}

func fromb64(s string) string {
	buf := bytes.NewBufferString(s)
	dec := base64.NewDecoder(base64.URLEncoding, buf)
	out := bytes.NewBufferString("")
	io.Copy(out, dec)
	return out.String()
}

// B64OrMD5HashEncodeKey converts a given key into a filesystem name-safe string
// and returns true iff it can be reversed with B64DecodeKey.
func B64OrMD5HashEncodeKey(key string) (string, bool) {
	b64key := tob64(key)
	// short name
	if len(b64key) < maxShort {
		return fmt.Sprintf("%s%s%s", shortPrefix, salt, b64key), true
	}

	// long name
	hash := md5.Sum([]byte(key))
	return fmt.Sprintf("%s%s%x", longPrefix, salt, hash[:]), false
}

func (fs *StandardFS) makeName(key string) (string, error) {
	name, decodable := fs.EncodeKey(key)
	if decodable {
		return name, nil
	}

	// Name is not decodeable, store it.
	f, err := fs.create(fmt.Sprintf("%s.key", name))
	if err != nil {
		return "", err
	}
	_, err = f.Write([]byte(key))
	f.Close()
	return name, err
}

// B64DecodeKey converts a string y into x st. y, ok = B64OrMD5HashEncodeKey(x), and ok = true.
// Basically it should reverse B64OrMD5HashEncodeKey if B64OrMD5HashEncodeKey returned true.
func B64DecodeKey(name string) (string, bool) {
	if strings.HasPrefix(name, shortPrefix) {
		return fromb64(strings.TrimPrefix(name, shortPrefix)[saltSize:]), true
	}
	return "", false
}

func (fs *StandardFS) getKey(name string) (string, error) {
	if key, ok := fs.DecodeKey(name); ok {
		return key, nil
	}

	// long name
	f, err := fs.Open(filepath.Join(fs.root, fmt.Sprintf("%s.key", name)))
	if err != nil {
		return "", err
	}
	defer f.Close()
	key, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(key), nil
}
