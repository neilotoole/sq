package drvr

import (
	"bytes"

	"os"
	"strings"

	"fmt"

	"io/ioutil"

	"net/url"

	"path/filepath"

	"mime"

	"io"
	"net/http"

	"github.com/hashicorp/go-getter"
	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/libsq/util"
)

// GenerateAlphaColName returns an Excel-style column name for index n, starting with A, B, C...
// and continuing to AA, AB, AC, etc...
func GenerateAlphaColName(n int) string {
	return genAlphaCol(n, 'A', 26)
}

func genAlphaCol(n int, start rune, lenAlpha int) string {

	buf := &bytes.Buffer{}

	for ; n >= 0; n = int(n/lenAlpha) - 1 {

		buf.WriteRune(rune(n%lenAlpha) + start)
	}

	return util.ReverseString(buf.String())
}

//// GetSourceFileName returns the final component of the file/URL path.
//func GetSourceFileName(src *drvr.Source) (string, error) {
//
//	sep := os.PathSeparator
//	if strings.HasPrefix(src.Location, "http") {
//		sep = '/'
//	}
//
//	// Why is this illegal? Should be ok to get a file from http root?
//	parts := strings.Split(src.Location, string(sep))
//	if len(parts) == 0 || len(parts[len(parts)-1]) == 0 {
//		return "", util.Errorf("illegal src [%s] location: %s", src.Handle, src.Location)
//	}
//
//	return parts[len(parts)-1], nil
//}

// GetSourceFile returns a file handle for the location, which can be a local filepath
// or a URL. If it's a remote file it will be downloaded to a temp file. If cleanup
// is non-nil, it should always be invoked, even if err is non-nil.
// The returned file is open and the caller is responsible for closing it.
func GetSourceFile(location string) (file *os.File, mediatype string, cleanup func() error, err error) {

	lg.Debugf(location)

	pwd, err := os.Getwd()
	if err != nil {
		return nil, "", nil, util.WrapError(err)
	}

	src, err := getter.Detect(location, pwd, getter.Detectors)
	if err != nil {
		return nil, "", nil, util.WrapError(err)
	}

	if strings.HasPrefix(src, "file://") {
		// It's a local file, we don't need to get it
		file, err = os.Open(location)
		if err != nil {
			return file, "", nil, util.WrapError(err)
		}

		mediatype = mime.TypeByExtension(filepath.Ext(file.Name()))
		return file, mediatype, nil, nil
	}

	// It's not a local file, we'll allow getter to fetch it
	srcURL, err := url.ParseRequestURI(src)
	if err != nil {
		// should never happen
		return nil, "", nil, util.WrapError(err)
	}

	// We want to save the fetched file to a temp file with the same name, but
	// it'll be called "download" if we can't determine the name.
	// TODO: should also look for the filename in the Content-Disposition
	dstFilename := "download"

	if srcURL.Path != "" {
		parts := strings.Split(srcURL.Path, "/")
		if len(parts) > 0 {
			name := parts[len(parts)-1]
			if name != "" {
				dstFilename = name
			}
		}
	}

	tmpDir, err := ioutil.TempDir("", "sq_download_")
	if err != nil {
		return nil, "", nil, util.WrapError(err)
	}

	cleanup = func() error {
		lg.Debugf("attempting to cleanup tmp dir: %s", tmpDir)
		return util.WrapError(os.RemoveAll(tmpDir))
	}

	dstFilepath := filepath.Join(tmpDir, dstFilename)

	mediatype, remoteFilename, err := getterGetFile(dstFilepath, src)
	if err != nil {
		return nil, "", cleanup, util.Errorf("failed to get file %q: %v", location, err)
	}

	if remoteFilename != "" && dstFilename != remoteFilename {
		// try to rename, but we don't really care if it doesn't work
		err2 := os.Rename(dstFilepath, filepath.Join(tmpDir, remoteFilename))
		lg.Warnf("failed to rename file to match remote name %q, but continuing regardless: %v", err2)
	}

	file, err = os.Open(dstFilepath)
	if err != nil {
		return file, "", nil, util.WrapError(err)
	}

	if mediatype == "" {
		mediatype = mime.TypeByExtension(filepath.Ext(file.Name()))
	}

	mt := "unknown media type"
	if mediatype != "" {
		mt = mediatype
	}

	lg.Debugf("downloaded [%s] file to: %s", mt, dstFilepath)

	return file, mediatype, cleanup, nil
}

// getterGetFile extends the behavior of getter.GetFile to also return the
// media type (from Content-Type / Content-Disposition) if it's a HTTP/HTTPS
// request, or "" if the type cannot be determined. If Content-Disposition specifies
// a file name, it will be returned in "filename" (the dst is not affected).
func getterGetFile(dst string, src string) (mediatype string, filename string, err error) {

	lg.Debugf("attempting to fetch %q to %q", src, dst)

	getters := make(map[string]getter.Getter)
	for typ, gtr := range getter.Getters {
		getters[typ] = gtr
	}

	httpGtr := &httpGetter{}
	httpGtr.Netrc = true

	getters["http"] = httpGtr
	getters["https"] = httpGtr

	err = (&getter.Client{
		Src:     src,
		Dst:     dst,
		Dir:     false,
		Getters: getters,
	}).Get()

	if err != nil {
		return "", "", util.WrapError(err)
	}

	if httpGtr.resp != nil {

		for _, hdr := range []string{"Content-Disposition", "Content-Type"} {
			val := httpGtr.resp.Header.Get(hdr)
			if val != "" {
				mt, params, err := mime.ParseMediaType(val)
				if err != nil {
					lg.Warnf("failed to parse %s header: %v", hdr, err)
					continue
				}

				name := params["filename"]
				if name != "" {
					filename = name
				}

				if mt != "" {
					mediatype = mt
					return mediatype, filename, nil
				}
			}
		}
	}

	return "", "", nil
}

// httpGetter extends getter.HttpGetter to allow capture of the response when
// invoke GetFile().
type httpGetter struct {
	resp *http.Response
	getter.HttpGetter
}

// GetFile is copied from getter.HttpGetter.GetFile, with a little hack added.
func (h *httpGetter) GetFile(dst string, u *url.URL) error {

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}

	h.resp = resp // this is our hack

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("bad response code: %d", resp.StatusCode)
	}

	// Create all the parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
