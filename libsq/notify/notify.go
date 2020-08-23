// Package notify is an experiment for sending notifications.
package notify

import (
	"regexp"
	"sync"
	"time"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// DestType is the destination type, e.g. "slack", "hipchat", or "email" etc.
type DestType string

// Destination is a destination for messages.
type Destination struct {
	Type        DestType `yaml:"type" json:"type"`
	Label       string   `yaml:"label" json:"label"`
	Target      string   `yaml:"target" json:"target"`
	Credentials string   `yaml:"credentials" json:"credentials"`
}

func (d Destination) String() string {
	return stringz.SprintJSON(d)
}

// Message is a notification message, optionally containing a Job that the message is associated with.
type Message struct {
	Text string `yaml:"text" json:"text"`
	Job  *Job   `yaml:"job,empty" json:"job,omitempty"`
}

// NewJobMessage creates a Message indicating the state of the job.
func NewJobMessage(jb Job) Message {
	m := Message{Job: &jb}
	return m
}

// Notifier is an interface that can send notification messages.
type Notifier interface {
	// Send sends the message.
	Send(msg Message) error
}

// Provider is a factory that returns Notifier instances and generates notification Destinations from user parameters.
type Provider interface {
	// Destination returns a notification Destination instance from the supplied parameters.
	Destination(typ DestType, target string, label string, credentials string, labelAvailable func(label string) bool) (*Destination, error)
	// Notifier returns a Notifier instance for the given destination.
	Notifier(dest Destination) (Notifier, error)
}

var providers = make(map[DestType]Provider)

// RegisterProvider should be invoked by notification implementations to indicate that they handle a specific destination type.
func RegisterProvider(typ DestType, p Provider) {
	providers[typ] = p
}

// ProviderFor returns a Provider for the specified destination type.
func ProviderFor(typ DestType) (Provider, error) {
	p, ok := providers[typ]
	if !ok {
		return nil, errz.Errorf("unsupported notification destination type %q", typ)
	}
	return p, nil
}

// NewAsyncNotifier returns a Notifier that sends messages asynchronously to the supplied destination.
// The invoking code should call AsyncNotifier.Wait() before exiting.
// TODO: Should take a context.Context param.
func NewAsyncNotifier(log lg.Log, dests []Destination) (*AsyncNotifier, error) {
	notifiers := make([]Notifier, len(dests))

	for i, dest := range dests {
		provider, ok := providers[dest.Type]
		if !ok {
			return nil, errz.Errorf("no provider for notification destination type %q", dest.Type)
		}

		notifier, err := provider.Notifier(dest)
		if err != nil {
			return nil, err
		}

		notifiers[i] = notifier
	}

	return &AsyncNotifier{log: log, dests: notifiers, wg: &sync.WaitGroup{}, done: make(chan struct{})}, nil
}

// AsyncNotifier is a Notifier that wraps a bunch of other
// notifiers and sends message asynchronously. The invoking code
// should call AsyncNotifier.Wait() before exiting.
type AsyncNotifier struct {
	log   lg.Log
	dests []Notifier
	done  chan struct{}
	wg    *sync.WaitGroup
}

func (a *AsyncNotifier) Send(msg Message) error {
	a.wg.Add(len(a.dests))

	for _, dest := range a.dests {
		dest := dest
		go func() {
			defer a.wg.Done()
			err := dest.Send(msg)
			if err != nil {
				a.log.Warnf("problem sending notification: %v", err)
			}
		}()
	}
	return nil
}

func (a *AsyncNotifier) Wait(timeout time.Duration) {
	go func() {
		a.wg.Wait()
		close(a.done)
	}()

	select {
	case <-a.done:
	case <-time.After(timeout):
		a.log.Warnf("hit timeout before all notifiers completed")
	}
}

var handlePattern = regexp.MustCompile(`\A[a-zA-Z][a-zA-Z0-9_]*$`)

// ValidHandle returns an error if handle is not an acceptable notification destination handle value.
func ValidHandle(handle string) error {
	if !handlePattern.MatchString(handle) {
		return errz.Errorf(`invalid notification destination handle value %q: must begin with a letter, followed by zero or more letters, digits, or underscores, e.g. "slack_devops"`, handle)
	}

	return nil
}
