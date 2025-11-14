package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var readmeContents string

// SetReadmeContents sets the README content from main package
func SetReadmeContents(content string) {
	readmeContents = content
}

// docsCmd represents the docs command
var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Display the lincli documentation",
	Long: `Display the complete lincli documentation from README.md.

This command outputs the full documentation in markdown format,
which can be piped to other tools or saved to a file.

Examples:
  lincli docs                    # Display documentation
  lincli docs | less            # View with pager
  lincli docs > lincli-docs.md  # Save to file`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(readmeContents)
	},
}

func init() {
	rootCmd.AddCommand(docsCmd)
}
