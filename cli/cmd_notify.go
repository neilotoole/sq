package cli

import (
	"fmt"

	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/notify"
)

func newNotifyCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:    "notify",
		Hidden: true, // Not advertising this feature right now
		Short:  "Manage notification destinations",
		Example: `sq notify ls
sq notify add slack devops [...]
sq notify rm devops
sq notify add --help`,
	}

	return cmd, func(rc *RunContext, cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}
}

func newNotifyListCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List notification destinations",
	}

	return cmd, execNotifyList
}

func execNotifyList(rc *RunContext, cmd *cobra.Command, args []string) error {
	return rc.writers.notifyw.NotifyDestinations(rc.Config.Notification.Destinations)
}

func newNotifyRemoveCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:     "rm",
		Aliases: []string{"remove"},
		Short:   "Remove notification destination",
	}

	return cmd, execNotifyRemove
}

func execNotifyRemove(rc *RunContext, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errz.Errorf("this command takes exactly one argument")
	}

	cfg := rc.Config

	if len(cfg.Notification.Destinations) == 0 {
		return errz.Errorf("the notification destination %q does not exist", args[0])
	}

	var dests []notify.Destination
	for _, dest := range cfg.Notification.Destinations {
		if dest.Label == args[0] {
			continue
		}
		dests = append(dests, dest)
	}

	if len(dests) == len(cfg.Notification.Destinations) {
		return errz.Errorf("the notification destination %q does not exist", args[0])
	}

	cfg.Notification.Destinations = dests
	err := rc.ConfigStore.Save(rc.Config)
	if err != nil {
		return err
	}

	return nil
}

func newNotifyAddCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add notification destination",
		Example: `sq notify add slack #devops xoxp-892529...911b8a
sq notify add slack --help
sq notify add hipchat myteam ABAD098ASDF...99AB
sq notify add hipchat --help
`,
	}

	return cmd, nil
}

func newNotifyAddSlackCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:   "slack CHANNEL [HANDLE] TOKEN",
		Short: "Add Slack channel",
		Long: `Add Slack channel. The CHANNEL param should not include the leading '#'.
	The HANDLE param is optional; if not provided, a handle
will be generated. To generate the auth token using your browser, login to
https://TEAM.slack.com and then visit https://api.slack.com/custom-integrations/legacy-tokens
and use the "Legacy token generator" to get the token value.`,
		Example: `sq notify add slack devops xoxp-892529...911b8a`,
	}

	return cmd, execNotifyAddSlack
}

func execNotifyAddSlack(rc *RunContext, cmd *cobra.Command, args []string) error {
	if len(args) != 2 && len(args) != 3 {
		return errz.Errorf(`this command takes either 2 or 3 arguments: see "sq notify add slack --help"`)
	}

	cfg := rc.Config

	labelAvailableFn := func(label string) bool {
		for _, dest := range cfg.Notification.Destinations {
			if dest.Label == label {
				return false
			}
		}
		return true
	}

	provider, err := notify.ProviderFor("slack")
	if err != nil {
		return err
	}

	target := args[0]
	var label string
	var token string

	if len(args) == 2 {
		token = args[1]
	} else {
		label = args[1]
		token = args[2]

		if !labelAvailableFn(label) {
			return errz.Errorf("a notifier with the label %q already exists", label)
		}

		err = notify.ValidHandle(label)
		if err != nil {
			return err
		}
	}

	dest, err := provider.Destination(notify.DestType("slack"), target, label, token, labelAvailableFn)
	if err != nil {
		return err
	}

	cfg.Notification.Destinations = append(cfg.Notification.Destinations, *dest)

	err = rc.ConfigStore.Save(rc.Config)
	if err != nil {
		return err
	}

	return rc.writers.notifyw.NotifyDestinations([]notify.Destination{*dest})
}

func newNotifyAddHipChatCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:     "hipchat ROOM TOKEN",
		Short:   "Add HipChat room",
		Example: `sq notify add hipchat devops --label="hip_devops" BOuyOe...VRBksq6`,
	}

	cmd.Flags().String(flagNotifierLabel, "", flagNotifierLabelUsage)

	return cmd, execNotifyAddHipChat
}

func execNotifyAddHipChat(rc *RunContext, cmd *cobra.Command, args []string) error {
	fmt.Fprintln(rc.Out, "Add HipChat room")
	fmt.Fprintln(rc.Out, strings.Join(args, " | "))

	var label string
	var err error
	if cmd.Flags().Changed(flagNotifierLabel) {
		label, err = cmd.Flags().GetString(flagNotifierLabel)
		if err != nil {
			return errz.Err(err)
		}
	}

	if label != "" {
		fmt.Fprintf(rc.Out, "Label: %s", label)
	}

	return nil
}
