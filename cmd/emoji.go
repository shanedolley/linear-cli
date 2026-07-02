package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Khan/genqlient/graphql"
	"github.com/fatih/color"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var emojiCmd = &cobra.Command{
	Use:   "emoji",
	Short: "Manage custom workspace emoji",
	Long: `List, create, and delete the workspace's custom emoji.

Examples:
  lincli emoji list
  lincli emoji create --name partyparrot --url https://example.com/parrot.gif
  lincli emoji delete partyparrot`,
}

var emojiListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List custom emoji",
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx := emojiClient(plaintext, jsonOut)

		limit, _ := cmd.Flags().GetInt("limit")
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		resp, err := api.ListEmojis(ctx, client, limitPtr, nil, nil)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list emoji: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if resp.Emojis == nil || len(resp.Emojis.Nodes) == 0 {
			output.Info("No custom emoji found", plaintext, jsonOut)
			return
		}

		if jsonOut {
			output.JSON(resp.Emojis.Nodes)
			return
		}

		headers := []string{"Name", "Source", "Creator", "ID"}
		rows := make([][]string, len(resp.Emojis.Nodes))
		for i, node := range resp.Emojis.Nodes {
			f := node.EmojiFields
			creator := "-"
			if f.Creator != nil {
				creator = f.Creator.Name
			}
			rows[i] = []string{f.Name, f.Source, truncateString(creator, 20), f.Id}
		}

		output.Table(output.TableData{Headers: headers, Rows: rows}, plaintext, jsonOut)

		if !plaintext && !jsonOut {
			fmt.Printf("\n%s %d emoji\n", color.New(color.FgGreen).Sprint("✓"), len(resp.Emojis.Nodes))
		}
	},
}

var emojiCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new"},
	Short:   "Create a custom emoji",
	Long: `Create a custom emoji. --name is required, plus exactly one image source:

  --file  a local image to upload (recommended)
  --url   a Linear-hosted asset URL (uploads.linear.app/...)

Linear rejects arbitrary external URLs, so --file uploads the image first and
uses the resulting asset URL.`,
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			output.Error("Name is required (--name)", plaintext, jsonOut)
			os.Exit(1)
		}
		url, _ := cmd.Flags().GetString("url")
		file, _ := cmd.Flags().GetString("file")
		if (url == "") == (file == "") {
			output.Error("Provide exactly one image source: --file or --url", plaintext, jsonOut)
			os.Exit(1)
		}

		client, ctx := emojiClient(plaintext, jsonOut)

		if file != "" {
			uploaded, err := uploadEmojiImage(ctx, client, file, !jsonOut && !plaintext)
			if err != nil {
				output.Error(err.Error(), plaintext, jsonOut)
				os.Exit(1)
			}
			url = uploaded
		}

		input := &api.EmojiCreateInput{Name: name, Url: url}

		resp, err := api.EmojiCreate(ctx, client, input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create emoji: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.EmojiCreate == nil || !resp.EmojiCreate.Success {
			output.Error("Failed to create emoji", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(resp.EmojiCreate.Emoji)
		} else {
			output.Success(fmt.Sprintf("Created emoji :%s:", resp.EmojiCreate.Emoji.EmojiFields.Name), plaintext, jsonOut)
		}
	},
}

var emojiDeleteCmd = &cobra.Command{
	Use:     "delete <name-or-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a custom emoji",
	Long:    `Delete a custom emoji by name or ID.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, ctx := emojiClient(plaintext, jsonOut)

		id, err := resolveEmoji(ctx, client, args[0])
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		resp, err := api.EmojiDelete(ctx, client, id)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete emoji: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}
		if resp.EmojiDelete == nil || !resp.EmojiDelete.Success {
			output.Error("Failed to delete emoji", plaintext, jsonOut)
			os.Exit(1)
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"success": true, "id": id})
		} else {
			output.Success(fmt.Sprintf("Deleted emoji %s", args[0]), plaintext, jsonOut)
		}
	},
}

// resolveEmoji resolves a custom emoji name or UUID to its ID. The single
// `emoji(id)` query accepts a name or an id, so a non-UUID reference is looked
// up through it to obtain the canonical id that emojiDelete requires.
func resolveEmoji(ctx context.Context, client *api.Client, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}
	resp, err := api.GetEmoji(ctx, client, nameOrID)
	if err != nil {
		return "", fmt.Errorf("Emoji not found: %s", nameOrID)
	}
	return resp.Emoji.EmojiFields.Id, nil
}

// uploadEmojiImage uploads a local image through Linear's file-upload flow and
// returns the resulting asset URL, which emojiCreate accepts (a raw external
// URL does not). It reuses the shared validate/detect/upload helpers from the
// attachment command.
func uploadEmojiImage(ctx context.Context, client graphql.Client, path string, showProgress bool) (string, error) {
	size, err := validateFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot upload %s: %w", path, err)
	}
	contentType, err := detectContentType(path)
	if err != nil {
		return "", fmt.Errorf("cannot determine file type of %s: %w", path, err)
	}

	uploadResp, err := api.FileUpload(ctx, client, contentType, filepath.Base(path), int(size))
	if err != nil {
		return "", fmt.Errorf("failed to get upload URL: %w", err)
	}
	uf := uploadResp.FileUpload.UploadFile
	if err := uploadFileWithProgress(path, uf.UploadUrl, uf.Headers, contentType, size, !showProgress); err != nil {
		return "", err
	}
	return uf.AssetUrl, nil
}

// emojiClient builds the authenticated client and context shared by the emoji
// subcommands, exiting on an auth failure.
func emojiClient(plaintext, jsonOut bool) (*api.Client, context.Context) {
	authHeader, err := auth.GetAuthHeader()
	if err != nil {
		output.Error(fmt.Sprintf("Authentication failed: %v", err), plaintext, jsonOut)
		os.Exit(1)
	}
	return api.NewClient(authHeader), context.Background()
}

func init() {
	rootCmd.AddCommand(emojiCmd)

	emojiCmd.AddCommand(emojiListCmd)
	emojiCmd.AddCommand(emojiCreateCmd)
	emojiCmd.AddCommand(emojiDeleteCmd)

	emojiListCmd.Flags().IntP("limit", "l", 50, "Maximum number of emoji to fetch")

	emojiCreateCmd.Flags().String("name", "", "Emoji name, without colons (required)")
	emojiCreateCmd.Flags().String("file", "", "Local image file to upload (use this or --url)")
	emojiCreateCmd.Flags().String("url", "", "Linear-hosted asset URL, uploads.linear.app/... (use this or --file)")
}
