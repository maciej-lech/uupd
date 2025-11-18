package main

import (
	"log"
	"os/exec"
	"strings"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"
	"github.com/spf13/cobra"
)

var (
	notifyCmd = &cobra.Command{
		Use:   "notify <summary> [body]",
		Short: "Send desktop notifications",
		Args:  cobra.RangeArgs(1, 2),
		Run:   sendNotification,
	}

	module         string
	urgency        string
	actions        []string
	actionCommands map[string]string
	closeChan      chan struct{}
)

func init() {
	rootCmd.AddCommand(notifyCmd)

	notifyCmd.Flags().StringVar(&module, "module", "", "Module name (e.g., firmware, system, flatpak)")
	notifyCmd.Flags().StringVar(&urgency, "urgency", "normal", "Urgency level (low, normal, critical)")
	notifyCmd.Flags().StringArrayVar(&actions, "action", []string{}, "Actions in format 'key=command'")
}

func parseActions(actions []string) ([]notify.Action, map[string]string) {
	var notifyActions []notify.Action
	actionCommands := make(map[string]string)

	for _, action := range actions {
		parts := strings.SplitN(action, "=", 2)
		if len(parts) != 2 {
			log.Fatalf("Invalid action format: %s (expected key=command)", action)
		}
		key := parts[0]
		command := parts[1]
		actionCommands[key] = command
		notifyActions = append(notifyActions, notify.Action{Key: key, Label: strings.Title(key)})
	}

	return notifyActions, actionCommands
}

func actionHandler(action *notify.ActionInvokedSignal) {
	if command, exists := actionCommands[action.ActionKey]; exists {
		cmdParts := strings.Fields(command)
		cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
		_ = cmd.Start()
	}
}

func closeHandler(closed *notify.NotificationClosedSignal) {
	close(closeChan)
}

func sendNotification(cmd *cobra.Command, args []string) {
	summary := args[0]
	body := ""
	if len(args) > 1 {
		body = args[1]
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		log.Fatalf("Failed to connect to session bus: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	urgencyLevel := notify.UrgencyNormal
	switch urgency {
	case "low":
		urgencyLevel = notify.UrgencyLow
	case "normal":
		urgencyLevel = notify.UrgencyNormal
	case "critical":
		urgencyLevel = notify.UrgencyCritical
	default:
		log.Fatalf("Invalid urgency level: %s", urgency)
	}

	var notifyActions []notify.Action
	notifyActions, actionCommands = parseActions(actions)

	notification := notify.Notification{
		AppName:       "uupd",
		ReplacesID:    0,
		Summary:       summary,
		Body:          body,
		Actions:       notifyActions,
		ExpireTimeout: 10,
	}
	notification.SetUrgency(urgencyLevel)

	closeChan = make(chan struct{})

	notifier, err := notify.New(
		conn,
		notify.WithOnAction(actionHandler),
		notify.WithOnClosed(closeHandler),
	)
	if err != nil {
		log.Fatalf("Failed to create notifier: %v", err)
	}
	defer notifier.Close() //nolint:errcheck

	_, err = notifier.SendNotification(notification)
	if err != nil {
		log.Fatalf("Failed to send notification: %v", err)
	}

	<-closeChan
}
