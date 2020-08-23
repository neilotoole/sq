package fetcher

import (
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-getter"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// FetchFile returns a file handle for the location, which can be a local filepath
// or a URL. If it's a remote location, the file will be downloaded to a temp file.
// If file is non-nil, cleanFn will be non-nil (but could be no-op).
// The returned file is open and the caller is responsible for closing it.
//
// Deprecated: use Fetcher.Fetch.
func FetchFile(log lg.Log, location string) (file *os.File, mediatype string, cleanFn func() error, err error) {
	log.Debugf("attempting to fetch file from %s", location)

	pwd, err := os.Getwd()
	if err != nil {
		return nil, "", nil, errz.Wrap(err, "failed to get working dir")
	}

	src, err := getter.Detect(location, pwd, getter.Detectors)
	if err != nil {
		return nil, "", nil, errz.Wrap(err, "failed to detect file")
	}

	if strings.HasPrefix(src, "file://") {
		// It's a local file, we don't need to get it
		file, err = os.Open(location)
		if err != nil {
			return nil, "", nil, errz.Wrap(err, "failed to open file")
		}

		mediatype = mime.TypeByExtension(filepath.Ext(file.Name()))
		return file, mediatype, nil, nil
	}

	// It's not a local file, we'll allow getter to fetch it
	srcURL, err := url.ParseRequestURI(src)
	if err != nil {
		// should never happen
		return nil, "", nil, errz.Wrap(err, "failed to parse source URL")
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
		return nil, "", nil, errz.Err(err)
	}

	cleanFn = func() error {
		log.Debugf("attempting to cleanup tmp dir used for download: %s", tmpDir)
		return errz.Wrap(os.RemoveAll(tmpDir), "failed to remove tmp dir")
	}

	dstFilepath := filepath.Join(tmpDir, dstFilename)

	mediatype, remoteFilename, err := getterGetFile(log, dstFilepath, src)
	if err != nil {
		log.WarnIfError(cleanFn())
		return nil, "", cleanFn, errz.Errorf("failed to get file %q: %v", location, err)
	}

	if remoteFilename != "" && dstFilename != remoteFilename {
		// try to rename, but we don't really care if it doesn't work
		// REVISIT: not sure what exactly this is supposed to be doing
		err2 := os.Rename(dstFilepath, filepath.Join(tmpDir, remoteFilename))
		if err2 != nil {
			log.Warnf("failed to rename file to match remote name %q, but continuing regardless: %v", err2)
		}
	}

	file, err = os.Open(dstFilepath)
	if err != nil {
		log.WarnIfError(cleanFn())
		return file, "", cleanFn, errz.Wrap(err, "failed to open file")
	}

	if mediatype == "" {
		mediatype = mime.TypeByExtension(filepath.Ext(file.Name()))
	}

	if mediatype == "" {
		log.Debugf("downloaded [unknown media type] file to: %s", dstFilepath)
	} else {
		log.Debugf("downloaded [%s] file to: %s", mediatype, dstFilepath)
	}

	return file, mediatype, cleanFn, nil
}

// getterGetFile extends the behavior of getter.GetFile to also return the
// media type (from Content-Datatype / Content-Disposition) if it's a HTTP/HTTPS
// request, or "" if the type cannot be determined. If Content-Disposition specifies
// a file name, it will be returned in "filename" (the dst is not affected).
func getterGetFile(log lg.Log, dst string, src string) (mediatype string, filename string, err error) {
	log.Debugf("attempting to fetch %q to %q", src, dst)

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
		return "", "", errz.Wrap(err, "failed to get file")
	}

	if httpGtr.resp != nil {

		for _, hdr := range []string{"Content-Disposition", "Content-Datatype"} {
			val := httpGtr.resp.Header.Get(hdr)
			if val != "" {
				mt, params, err := mime.ParseMediaType(val)
				if err != nil {
					log.Warnf("failed to parse %s header: %v", hdr, err)
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
		return errz.Wrapf(err, "")
	}

	h.resp = resp // this is our hack

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errz.Errorf("bad response code: %d", resp.StatusCode)
	}

	// Create all the parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return errz.Wrapf(err, "")
	}

	f, err := os.Create(dst)
	if err != nil {
		return errz.Err(err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return errz.Err(err)
}
