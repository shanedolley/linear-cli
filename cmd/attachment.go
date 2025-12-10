package cmd

import (
	"github.com/spf13/cobra"
)

// attachmentCmd represents the attachment command
var attachmentCmd = &cobra.Command{
	Use:   "attachment",
	Short: "Manage issue attachments",
	Long: `Manage attachments on Linear issues.

Supports both file uploads to Linear's storage and URL attachments.
Use subcommands to list, create, update, and delete attachments.`,
}

func init() {
	rootCmd.AddCommand(attachmentCmd)
}
