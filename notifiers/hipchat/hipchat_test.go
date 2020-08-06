package hipchat

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/notify"
)

const testToken = `Q1dZ4yFDWGMJxt0u9kiBA0yoCNm5Ga0wvs7PO7Uk`
const testRoomName = `notifier-test`
const testRoomID = `3836173`
const baseURL = `https://api.hipchat.com/v2/`

func TestProvider_Destination(t *testing.T) {
	t.Skip()
	available := func(string) bool {
		return true
	}
	target := fmt.Sprintf("%sroom/%s", baseURL, testRoomName)
	p := &provider{}

	dest, err := p.Destination(
		typ,
		target,
		"",
		testToken,
		available)

	require.Nil(t, err)
	require.NotNil(t, dest)
}

func TestNotifier_Send(t *testing.T) {
	t.Skip()
	p := &provider{}
	dest := newTestDestination(t, p)

	n, err := p.Notifier(*dest)

	require.Nil(t, err)
	require.NotNil(t, n)

	err = n.Send(notify.Message{Text: "hello world"})
	require.Nil(t, err)

	jb := notify.New("@my1.tbluser | .uid, .username, .email")
	jb.Start()
	err = n.Send(notify.NewJobMessage(*jb))
	require.Nil(t, err)
	jb.Fail()
	err = n.Send(notify.NewJobMessage(*jb))
	require.Nil(t, err)

	jb = notify.New("@my1.tbluser | .uid, .username, .email")
	jb.Start()
	err = n.Send(notify.NewJobMessage(*jb))
	require.Nil(t, err)
	jb.Complete()
	err = n.Send(notify.NewJobMessage(*jb))
	require.Nil(t, err)

}

func newTestDestination(t *testing.T, p *provider) *notify.Destination {

	available := func(string) bool {
		return true
	}
	target := fmt.Sprintf("%sroom/%s", baseURL, testRoomName)
	dest, err := p.Destination(
		typ,
		target,
		"",
		testToken,
		available)

	require.Nil(t, err)
	return dest
}

//   https://api.hipchat.com/v2/room/12345  ->  "https://api.hipchat.com/v2/", "12345"
//   https://hipchat.acme.com/v2/room/12345  ->  "https://hipchat.acme.com/v2/", "12345"
func Test_parseTargetURL(t *testing.T) {
	t.Skip()
	base, id, err := parseTargetURL("https://api.hipchat.com/v2/room/12345")
	require.Nil(t, err)
	require.Equal(t, "https://api.hipchat.com/v2/", base)
	require.Equal(t, "12345", id)

	_, _, err = parseTargetURL("https://api.hipchat.com/v1/room/12345")
	require.NotNil(t, err, "only v2 API supported")
	_, _, err = parseTargetURL("https://api.hipchat.com/v3/room/12345")
	require.NotNil(t, err, "only v3 API supported")

	base, id, err = parseTargetURL("https://hipchat.acme.com/v2/room/45678")
	require.Nil(t, err)
	require.Equal(t, "https://hipchat.acme.com/v2/", base)
	require.Equal(t, "45678", id)
}

func TestProvider_generateTargetURL(t *testing.T) {
	t.Skip()
	val := generateTargetURL("https://api.hipchat.com/v2/", "12345")
	require.Equal(t, "https://api.hipchat.com/v2/room/12345", val)
	val = generateTargetURL("https://hipchat.acme.com/v2/", "45678")
	require.Equal(t, "https://hipchat.acme.com/v2/room/45678", val)
}
