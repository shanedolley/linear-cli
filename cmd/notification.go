package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var notificationCmd = &cobra.Command{
	Use:     "notification",
	Aliases: []string{"notif"},
	Short:   "Manage your notifications",
	Long: `Manage your inbox notifications.

Examples:
  lincli notification list
  lincli notification read-all
  lincli notification unread-all
  lincli notification archive NOTIFICATION-ID
  lincli notification snooze NOTIFICATION-ID --until 2026-08-01
  lincli notification subscribe --project "Q1 Roadmap"
  lincli notification unsubscribe SUBSCRIPTION-ID`,
}

var notificationListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List notifications (shows unread count)",
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := notificationClient()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		unreadResp, err := api.NotificationsUnreadCount(ctx, client)
		if err != nil {
			return fmt.Errorf("Failed to get unread count: %v", err)
		}
		unread := unreadResp.NotificationsUnreadCount

		resp, err := api.ListNotifications(ctx, client, nil, limitPtr, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list notifications: %v", err)
		}

		nodes := []api.ListNotificationsNotificationsNotificationConnectionNodesNotification{}
		if resp.Notifications != nil {
			nodes = resp.Notifications.Nodes
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"unreadCount": unread, "notifications": nodes})
			return nil
		}

		if len(nodes) == 0 {
			output.Info(fmt.Sprintf("No notifications found (%d unread)", unread), plaintext, jsonOut)
			return nil
		}

		headers := []string{"ID", "Type", "Title", "Read", "Created"}
		rows := make([][]string, len(nodes))
		for i, node := range nodes {
			read := "no"
			if node.GetReadAt() != nil {
				read = "yes"
			}
			rows[i] = []string{
				node.GetId(),
				node.GetType(),
				truncateString(node.GetTitle(), 40),
				read,
				node.GetCreatedAt().Format("2006-01-02"),
			}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext {
			fmt.Printf("\n%s %d notifications, %s unread\n",
				color.New(color.FgGreen).Sprint("✓"),
				len(nodes),
				color.New(color.FgYellow).Sprintf("%d", unread))
		}
		return nil
	},
}

var notificationReadAllCmd = &cobra.Command{
	Use:   "read-all",
	Short: "Mark notifications as read",
	Long: `Mark notifications as read. With no flags this targets the whole inbox;
--issue scopes the action to a single issue's notifications.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := notificationClient()
		if err != nil {
			return err
		}
		input := notificationEntityInput(cmd)

		resp, err := api.NotificationMarkReadAll(ctx, client, input, time.Now())
		if err != nil {
			return fmt.Errorf("Failed to mark notifications read: %v", err)
		}
		if resp.NotificationMarkReadAll == nil || !resp.NotificationMarkReadAll.Success {
			return errors.New("Failed to mark notifications read")
		}
		output.Success("Marked notifications as read", plaintext, jsonOut)
		return nil
	},
}

var notificationUnreadAllCmd = &cobra.Command{
	Use:   "unread-all",
	Short: "Mark notifications as unread",
	Long: `Mark notifications as unread. With no flags this targets the whole inbox;
--issue scopes the action to a single issue's notifications.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := notificationClient()
		if err != nil {
			return err
		}
		input := notificationEntityInput(cmd)

		resp, err := api.NotificationMarkUnreadAll(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to mark notifications unread: %v", err)
		}
		if resp.NotificationMarkUnreadAll == nil || !resp.NotificationMarkUnreadAll.Success {
			return errors.New("Failed to mark notifications unread")
		}
		output.Success("Marked notifications as unread", plaintext, jsonOut)
		return nil
	},
}

var notificationArchiveCmd = &cobra.Command{
	Use:   "archive <notification-id>",
	Short: "Archive a notification",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := notificationClient()
		if err != nil {
			return err
		}

		resp, err := api.NotificationArchive(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to archive notification: %v", err)
		}
		if resp.NotificationArchive == nil || !resp.NotificationArchive.Success {
			return errors.New("Failed to archive notification")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0]})
		} else {
			output.Success(fmt.Sprintf("Archived notification %s", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

var notificationSnoozeCmd = &cobra.Command{
	Use:   "snooze <notification-id>",
	Short: "Snooze a notification until a given time",
	Long:  `Snooze a notification until --until (a YYYY-MM-DD date or an RFC3339 timestamp).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		until, _ := cmd.Flags().GetString("until")
		if until == "" {
			return errors.New("Snooze time is required (--until)")
		}
		snoozeUntil, err := parseSnoozeTime(until)
		if err != nil {
			return err
		}

		client, ctx, err := notificationClient()
		if err != nil {
			return err
		}

		input := &api.NotificationUpdateInput{SnoozedUntilAt: &snoozeUntil}
		resp, err := api.NotificationUpdate(ctx, client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to snooze notification: %v", err)
		}
		if resp.NotificationUpdate == nil || !resp.NotificationUpdate.Success {
			return errors.New("Failed to snooze notification")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0], "snoozedUntil": snoozeUntil})
		} else {
			output.Success(fmt.Sprintf("Snoozed notification %s until %s", args[0], snoozeUntil.Format("2006-01-02 15:04")), plaintext, jsonOut)
		}
		return nil
	},
}

var notificationArchiveAllCmd = &cobra.Command{
	Use:   "archive-all",
	Short: "Archive notifications",
	Long: `Archive notifications. With no flags this targets the whole inbox;
--issue scopes the action to a single issue's notifications.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := notificationClient()
		if err != nil {
			return err
		}
		input := notificationEntityInput(cmd)

		resp, err := api.NotificationArchiveAll(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to archive notifications: %v", err)
		}
		if resp.NotificationArchiveAll == nil || !resp.NotificationArchiveAll.Success {
			return errors.New("Failed to archive notifications")
		}
		output.Success("Archived notifications", plaintext, jsonOut)
		return nil
	},
}

var notificationSnoozeAllCmd = &cobra.Command{
	Use:   "snooze-all",
	Short: "Snooze notifications until a given time",
	Long: `Snooze notifications until --until (a YYYY-MM-DD date or RFC3339 timestamp).
With no other flags this targets the whole inbox; --issue scopes the action to
a single issue's notifications.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		until, _ := cmd.Flags().GetString("until")
		if until == "" {
			return errors.New("Snooze time is required (--until)")
		}
		snoozeUntil, err := parseSnoozeTime(until)
		if err != nil {
			return err
		}

		client, ctx, err := notificationClient()
		if err != nil {
			return err
		}
		input := notificationEntityInput(cmd)

		resp, err := api.NotificationSnoozeAll(ctx, client, input, snoozeUntil)
		if err != nil {
			return fmt.Errorf("Failed to snooze notifications: %v", err)
		}
		if resp.NotificationSnoozeAll == nil || !resp.NotificationSnoozeAll.Success {
			return errors.New("Failed to snooze notifications")
		}
		output.Success(fmt.Sprintf("Snoozed notifications until %s", snoozeUntil.Format("2006-01-02 15:04")), plaintext, jsonOut)
		return nil
	},
}

var notificationSubscribeCmd = &cobra.Command{
	Use:   "subscribe",
	Short: "Subscribe to notifications for an entity",
	Long: `Subscribe to notifications for exactly one entity, selected by a target flag:
--project, --team, --cycle, --label, --user, --initiative, or --view.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := notificationClient()
		if err != nil {
			return err
		}
		cache := newResolverCache()

		input, err := buildSubscriptionInput(ctx, client, cache, cmd)
		if err != nil {
			return err
		}

		resp, err := api.NotificationSubscriptionCreate(ctx, client, input)
		if err != nil {
			return fmt.Errorf("Failed to subscribe: %v", err)
		}
		if resp.NotificationSubscriptionCreate == nil || !resp.NotificationSubscriptionCreate.Success {
			return errors.New("Failed to subscribe")
		}

		if jsonOut {
			output.JSON(resp.NotificationSubscriptionCreate.NotificationSubscription)
		} else {
			output.Success(fmt.Sprintf("Subscribed (subscription %s)", resp.NotificationSubscriptionCreate.NotificationSubscription.GetId()), plaintext, jsonOut)
		}
		return nil
	},
}

var notificationUnsubscribeCmd = &cobra.Command{
	Use:   "unsubscribe <subscription-id>",
	Short: "Unsubscribe from an entity's notifications",
	Long: `Deactivate a notification subscription by ID (sets active=false, avoiding the
deprecated delete). Get the ID from the subscription created by 'subscribe'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx, err := notificationClient()
		if err != nil {
			return err
		}

		active := false
		input := &api.NotificationSubscriptionUpdateInput{Active: &active}
		resp, err := api.NotificationSubscriptionUpdate(ctx, client, args[0], input)
		if err != nil {
			return fmt.Errorf("Failed to unsubscribe: %v", err)
		}
		if resp.NotificationSubscriptionUpdate == nil || !resp.NotificationSubscriptionUpdate.Success {
			return errors.New("Failed to unsubscribe")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": args[0], "active": false})
		} else {
			output.Success(fmt.Sprintf("Unsubscribed (subscription %s)", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

// notificationClient builds the authenticated client and context shared by the
// notification subcommands.
func notificationClient() (graphql.Client, context.Context, error) {
	client, err := newGraphQLClient()
	if err != nil {
		return nil, nil, err
	}
	return client, context.Background(), nil
}

// notificationEntityInput builds the NotificationEntityInput for the batch
// mark/archive mutations. An --issue value scopes the action to that issue;
// with no flag the input is empty, targeting the caller's whole inbox.
func notificationEntityInput(cmd *cobra.Command) *api.NotificationEntityInput {
	input := &api.NotificationEntityInput{}
	if issue, _ := cmd.Flags().GetString("issue"); issue != "" {
		input.IssueId = &issue
	}
	return input
}

// buildSubscriptionInput resolves exactly one target flag into a
// NotificationSubscriptionCreateInput. Zero or multiple targets is an error,
// matching Linear's "exactly one target entity" rule.
func buildSubscriptionInput(ctx context.Context, client graphql.Client, cache *ResolverCache, cmd *cobra.Command) (*api.NotificationSubscriptionCreateInput, error) {
	input := &api.NotificationSubscriptionCreateInput{}
	targets := 0

	if v, _ := cmd.Flags().GetString("project"); v != "" {
		id, err := resolveProject(ctx, client, cache, v)
		if err != nil {
			return nil, err
		}
		input.ProjectId = &id
		targets++
	}
	if v, _ := cmd.Flags().GetString("team"); v != "" {
		id, err := resolveTeam(ctx, client, cache, v)
		if err != nil {
			return nil, err
		}
		input.TeamId = &id
		targets++
	}
	if v, _ := cmd.Flags().GetString("label"); v != "" {
		id, err := resolveLabel(ctx, client, cache, v)
		if err != nil {
			return nil, err
		}
		input.LabelId = &id
		targets++
	}
	if v, _ := cmd.Flags().GetString("user"); v != "" {
		id, err := resolveUser(ctx, client, cache, v)
		if err != nil {
			return nil, err
		}
		input.UserId = &id
		targets++
	}
	if v, _ := cmd.Flags().GetString("initiative"); v != "" {
		id, err := resolveInitiative(ctx, client, cache, v)
		if err != nil {
			return nil, err
		}
		input.InitiativeId = &id
		targets++
	}
	if v, _ := cmd.Flags().GetString("cycle"); v != "" {
		input.CycleId = &v
		targets++
	}
	if v, _ := cmd.Flags().GetString("view"); v != "" {
		input.CustomViewId = &v
		targets++
	}

	if targets == 0 {
		return nil, fmt.Errorf("a target is required: one of --project, --team, --cycle, --label, --user, --initiative, or --view")
	}
	if targets > 1 {
		return nil, fmt.Errorf("specify exactly one target entity, not several")
	}
	return input, nil
}

// parseSnoozeTime parses a --until value as a YYYY-MM-DD date (interpreted as
// local midnight, so "snooze until Aug 1" means end of Jul 31 in the caller's
// timezone rather than a UTC boundary that can land a day early) or an RFC3339
// timestamp, returning the target time.
func parseSnoozeTime(s string) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --until value %q: use YYYY-MM-DD or an RFC3339 timestamp", s)
}

func init() {
	rootCmd.AddCommand(notificationCmd)
	notificationCmd.AddCommand(notificationListCmd)
	notificationCmd.AddCommand(notificationReadAllCmd)
	notificationCmd.AddCommand(notificationUnreadAllCmd)
	notificationCmd.AddCommand(notificationArchiveCmd)
	notificationCmd.AddCommand(notificationArchiveAllCmd)
	notificationCmd.AddCommand(notificationSnoozeCmd)
	notificationCmd.AddCommand(notificationSnoozeAllCmd)
	notificationCmd.AddCommand(notificationSubscribeCmd)
	notificationCmd.AddCommand(notificationUnsubscribeCmd)

	notificationListCmd.Flags().IntP("limit", "l", 50, "Maximum number of notifications to fetch")

	notificationReadAllCmd.Flags().String("issue", "", "Scope to a single issue's notifications (ID)")
	notificationUnreadAllCmd.Flags().String("issue", "", "Scope to a single issue's notifications (ID)")
	notificationArchiveAllCmd.Flags().String("issue", "", "Scope to a single issue's notifications (ID)")

	notificationSnoozeCmd.Flags().String("until", "", "Snooze until this time: YYYY-MM-DD (local) or RFC3339 (required)")
	notificationSnoozeAllCmd.Flags().String("until", "", "Snooze until this time: YYYY-MM-DD (local) or RFC3339 (required)")
	notificationSnoozeAllCmd.Flags().String("issue", "", "Scope to a single issue's notifications (ID)")

	notificationSubscribeCmd.Flags().String("project", "", "Subscribe to a project (name or ID)")
	notificationSubscribeCmd.Flags().StringP("team", "t", "", "Subscribe to a team (key, name, or ID)")
	notificationSubscribeCmd.Flags().String("cycle", "", "Subscribe to a cycle (ID)")
	notificationSubscribeCmd.Flags().String("label", "", "Subscribe to a label (name or ID)")
	notificationSubscribeCmd.Flags().String("user", "", "Subscribe to a user (email, name, or ID)")
	notificationSubscribeCmd.Flags().String("initiative", "", "Subscribe to an initiative (name or ID)")
	notificationSubscribeCmd.Flags().String("view", "", "Subscribe to a custom view (ID)")
}
