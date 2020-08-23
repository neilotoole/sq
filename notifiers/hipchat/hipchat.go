// Package hipchat is deprecated.
// Deprecated:  don't use me
package hipchat

import (
	"strconv"

	"net/url"

	"fmt"

	"strings"

	"math"

	"bytes"

	"time"

	"github.com/tbruyelle/hipchat-go/hipchat"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/notify"
)

const typ = notify.DestType("hipchat")

func init() {
	notify.RegisterProvider(typ, &provider{})
}

type provider struct {
}

func (p *provider) Destination(
	typ notify.DestType,
	target string,
	label string,
	credentials string,
	labelAvailable func(label string) bool) (*notify.Destination, error) {

	if typ != typ {
		return nil, errz.Errorf("hipchat: unsupported destination type %q", typ)
	}

	api := hipchat.NewClient(credentials)
	foundRoom, baseURL, err := getRoom(api, target)
	if err != nil {
		return nil, err
	}
	canonicalTarget := generateTargetURL(baseURL, strconv.Itoa(foundRoom.ID))

	dest := &notify.Destination{Type: typ, Target: canonicalTarget, Credentials: credentials, Label: label}
	if dest.Label == "" {
		// need to generate the handle
		h := foundRoom.Name
		// check if we can just grab the room name
		if labelAvailable(h) {
			dest.Label = h
			return dest, nil
		}

		for i := 1; i < math.MaxInt32; i++ {
			h = fmt.Sprintf("%s_%d", foundRoom.Name, i)
			if labelAvailable(h) {
				dest.Label = h
				return dest, nil
			}
		}

		return nil, errz.Errorf("hipchat: unable to suggest label for %q notification destination %q", typ, target)
	}

	// label was provided by the user, check if it's legal
	err = notify.ValidHandle(label)
	if err != nil {
		return nil, errz.Wrap(err, "hipchat")
	}

	return dest, nil
}

func getRoom(api *hipchat.Client, target string) (*hipchat.Room, string, error) {

	opt := &hipchat.RoomsListOptions{IncludePrivate: true, IncludeArchived: true}
	rooms, _, err := api.Room.List(opt)
	if err != nil {
		return nil, "", errz.Wrapf(err, "hipchat")
	}

	baseURL, providedRoomID, err := parseTargetURL(target)
	if err != nil {
		return nil, "", err
	}

	var foundRoom *hipchat.Room
	intRoomID, err := strconv.Atoi(providedRoomID)
	if err != nil {
		// providedRoomID is not an int, hopefully it's the room name
		for _, r := range rooms.Items {
			if r.Name == providedRoomID {
				foundRoom = &r
				break
			}
		}
	} else {
		// providedRoomID is an int, it should match room.ID
		for _, r := range rooms.Items {
			if r.ID == intRoomID {
				foundRoom = &r
				break
			}
		}
	}

	if foundRoom == nil {
		return nil, "", errz.Errorf("hipchat: unable to find room matching target %q", target)
	}

	return foundRoom, baseURL, nil
}

func (p *provider) Notifier(dest notify.Destination) (notify.Notifier, error) {
	if dest.Type != typ {
		return nil, errz.Errorf("hipchat: unsupported destination type %q", dest.Type)
	}
	api := hipchat.NewClient(dest.Credentials)
	room, _, err := getRoom(api, dest.Target)
	if err != nil {
		return nil, err
	}

	return &notifier{dest: dest, api: api, room: room}, nil
}

// parseTargetURL returns the base URL and room ID from a HipChat target URL. For example:
//
//   https://api.hipchat.com/v2/room/12345  ->  "https://api.hipchat.com/v2/", "12345"
//   https://hipchat.acme.com/v2/room/12345  ->  "https://hipchat.acme.com/v2/", "12345"
func parseTargetURL(targetURL string) (baseURL, roomID string, err error) {

	u, err := url.ParseRequestURI(targetURL)
	if err != nil {
		return "", "", errz.Wrapf(err, "hipchat: unable to parse target URL %q", targetURL)
	}

	// e.g. "/v2/room/12345" -> [ "", "v2", "room", "12345"]
	pathParts := strings.Split(u.EscapedPath(), "/")

	if len(pathParts) < 4 {
		return "", "", errz.Errorf("hipchat: the API URL path should have at least 3 parts, but was: %s", targetURL)
	}
	if pathParts[1] != "v2" {
		return "", "", errz.Errorf(`hipchat: only the v2 API is supported, but API  URL was: %s`, targetURL)
	}

	return fmt.Sprintf("https://%s/v2/", u.Host), pathParts[3], nil
}

// generateTargetURL returns a canonical URL identifier for a hipchat room
func generateTargetURL(baseURL string, roomID string) string {
	return fmt.Sprintf("%sroom/%s", baseURL, roomID)
}

type notifier struct {
	dest notify.Destination
	api  *hipchat.Client
	room *hipchat.Room
}

const tplJobEnded = `<table>
<tr><th>Job ID</th><td><code>%s</code></td></tr>
<tr><th>State</th><td><strong>%s<strong></td></tr>
<tr><th>Started</th><td>%s</td></tr>
<tr><th>Duration</th><td>%s</td></tr>
</table>
<pre>%s</pre>
`

const tplJobNotEnded = `<table>
<tr><th>Job ID</th><td><code>%s</code></td></tr>
<tr><th>State</th><td><strong>%s</strong></td></tr>
<tr><th>Started</th><td>%s</td></tr>
</table>
<pre>%s</pre>
`

func (n *notifier) Send(msg notify.Message) error {

	req := &hipchat.NotificationRequest{Message: msg.Text}
	req.MessageFormat = "html"
	req.From = "sq bot"

	buf := &bytes.Buffer{}

	buf.WriteString(msg.Text)

	if msg.Job != nil {
		if buf.Len() > 0 {
			buf.WriteString("<br />")
		}

		switch msg.Job.State {
		case notify.Completed:
			req.Color = hipchat.ColorGreen
		case notify.Failed:
			req.Color = hipchat.ColorRed
		}

		var started string
		var duration string

		if msg.Job.Started == nil {
			started = "-"
		} else {
			started = msg.Job.Started.Format(time.RFC3339)
		}

		if msg.Job.Ended == nil {
			html := fmt.Sprintf(tplJobNotEnded, msg.Job.ID, msg.Job.State, started, msg.Job.Stmt)
			buf.WriteString(html)
		} else {
			duration = msg.Job.Ended.Sub(*msg.Job.Started).String()
			html := fmt.Sprintf(tplJobEnded, msg.Job.ID, msg.Job.State, started, duration, msg.Job.Stmt)
			buf.WriteString(html)
		}
	}

	req.Message = buf.String()
	_, err := n.api.Room.Notification(strconv.Itoa(n.room.ID), req)
	if err != nil {
		return errz.Wrap(err, "hipchat")
	}

	return nil
}
