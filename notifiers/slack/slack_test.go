package slack

import (
	"testing"

	"fmt"
	"time"

	"github.com/nlopes/slack"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/notify"
)

func TestNotifier_Send(t *testing.T) {
	t.Skip()
	p := &provider{}
	d := newTestDestination(t, p)
	n, err := p.Notifier(*d)
	require.Nil(t, err)
	slackNotifier, ok := n.(*notifier)
	require.True(t, ok)

	text := fmt.Sprintf("hello world @ %d", time.Now().Unix())
	err = slackNotifier.Send(message(text))
	require.Nil(t, err)

	slackMsg, err := getLatestMessage(slackNotifier)
	require.Nil(t, err)
	require.Equal(t, text, slackMsg.Text)
}

func message(text string) notify.Message {
	return notify.Message{Text: text}
}

func newTestDestination(t *testing.T, p *provider) *notify.Destination {

	available := func(handle string) bool {
		return true
	}
	dest, err := p.Destination(
		typ,
		"notifier-test",
		"",
		"xoxp-89252929845-89246861280-181096637445-5978e0477c04316bcf348795fc911b8a",
		available)
	require.Nil(t, err)
	return dest
}

func getLatestMessage(n *notifier) (*slack.Message, error) {
	channel, err := n.getChannel()
	if err != nil {
		return nil, err
	}

	historyParams := slack.HistoryParameters{}

	history, err := n.api.GetChannelHistory(channel.ID, historyParams)
	if err != nil {
		return nil, errz.Err(err)
	}

	if len(history.Messages) == 0 {
		return nil, errz.Errorf("no messages in history for channel %q", n.dest.Label)
	}
	return &history.Messages[len(history.Messages)-1], nil
}
