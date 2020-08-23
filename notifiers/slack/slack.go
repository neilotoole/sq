// Package slack implements notifications for Slack.
package slack

import (
	"fmt"
	"math"
	"time"

	"github.com/nlopes/slack"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/notify"
)

const typ = notify.DestType("slack")

func init() {
	notify.RegisterProvider(typ, &provider{})
}

type notifier struct {
	dest    notify.Destination
	api     *slack.Client
	channel *slack.Channel
	// threads maps Job IDs to slack timestamps/threads
	threads map[string]string
}

func (n *notifier) Send(msg notify.Message) error {
	_, err := n.getChannel()
	if err != nil {
		return err
	}

	// threadTS holds the slack thread timestamp.
	var threadTS string

	params := slack.PostMessageParameters{}

	params.AsUser = false
	params.Username = "sq bot"

	if msg.Job != nil {
		// let's check if there's already a thread for this Job ID
		var ok bool
		threadTS, ok = n.threads[msg.Job.ID]
		if ok {
			// there's already a thread associated with this Job ID; mark the params to indicate this
			params.ThreadTimestamp = threadTS
		}

		att := slack.Attachment{}

		if msg.Text == "" {
			params.Markdown = true
			att.Pretext = "Job " + string(msg.Job.State)
		}

		att.MarkdownIn = []string{"fields", "pretext"}

		switch msg.Job.State {
		case notify.Completed:
			att.Color = "good"
		case notify.Failed:
			att.Color = "danger"
		}

		var started string
		var duration string

		if msg.Job.Started == nil {
			started = "-"
		} else {
			started = fmt.Sprintf("<!date^%d^{date_num} {time_secs}|%s>", msg.Job.Started.Unix(), msg.Job.Started.Format(time.RFC3339))
		}

		if msg.Job.Ended == nil {
			duration = "-"
		} else {
			duration = msg.Job.Ended.Sub(*msg.Job.Started).String()
		}

		att.Fields = []slack.AttachmentField{
			{Title: "Job ID", Value: fmt.Sprintf("`%s`", msg.Job.ID), Short: true},
			{Title: "State", Value: string(msg.Job.State), Short: true},
			{Title: "Query", Value: fmt.Sprintf("```%s```", msg.Job.Stmt)},
			{Title: "Started", Value: started, Short: true},
			{Title: "Duration", Value: duration, Short: true},
		}

		params.Attachments = append(params.Attachments, att)

	}

	_, ts, err := n.api.PostMessage(n.channel.ID, msg.Text, params)
	if err != nil {
		return errz.Errorf(
			"error sending message to Slack channel %q (%q): %v",
			n.channel.Name,
			n.dest.Label,
			err)
	}

	if msg.Job != nil {
		// if threadTS is empty, then this msg is the first in this thread
		if threadTS == "" {
			// associate the timestamp with the Job ID
			n.threads[msg.Job.ID] = ts
		}
	}

	return nil
}

func (n *notifier) getChannel() (*slack.Channel, error) {

	if n.channel != nil {
		return n.channel, nil
	}

	channels, err := n.api.GetChannels(true)
	if err != nil {
		return nil, errz.Err(err)
	}

	for _, c := range channels {
		if c.ID == n.dest.Target {
			n.channel = &c
			return n.channel, nil
		}
	}

	return nil, errz.Errorf("did not find Slack channel [%s] for notification destination %q", n.dest.Target, n.dest.Label)
}

func getChannel(channelName string, token string) (*slack.Channel, error) {

	api := slack.New(token)
	channels, err := api.GetChannels(true)
	if err != nil {
		return nil, errz.Err(err)
	}
	for _, c := range channels {

		if c.Name == channelName {
			return &c, nil
		}
	}

	return nil, errz.Errorf("did not find Slack channel %q", channelName)

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
		return nil, errz.Errorf("unsupported destination type %q", typ)
	}

	api := slack.New(credentials)
	team, err := api.GetTeamInfo()
	if err != nil {
		return nil, errz.Err(err)
	}

	channel, err := getChannel(target, credentials)
	if err != nil {
		return nil, err
	}

	dest := &notify.Destination{Type: typ, Target: channel.ID, Label: label, Credentials: credentials}
	if dest.Label == "" {
		// need to generate the handle
		h := fmt.Sprintf("%s_%s", team.Name, target)
		if labelAvailable(h) {
			dest.Label = h
			return dest, nil
		}

		for i := 1; i < math.MaxInt32; i++ {
			h = fmt.Sprintf("%s_%s_%d", team.Name, target, i)
			if labelAvailable(h) {
				dest.Label = h
				return dest, nil
			}
		}

		return nil, errz.Errorf("unable to suggest handle for %q notification destination %q", typ, target)
	}

	err = notify.ValidHandle(label)
	if err != nil {
		return nil, err
	}

	return dest, nil
}

func (p *provider) Notifier(dest notify.Destination) (notify.Notifier, error) {
	if dest.Type != typ {
		return nil, errz.Errorf("unsupported destination type %q", dest.Type)
	}
	return &notifier{dest: dest, api: slack.New(dest.Credentials), threads: make(map[string]string)}, nil
}
