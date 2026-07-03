package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// reactionInfo is a resolver-local, query-agnostic view of a reaction. The
// issue and comment reaction queries generate distinct node types, so both
// are normalized to this shape before matching in findReactionID.
type reactionInfo struct {
	id     string
	emoji  string
	userID string
}

// emojiAliases maps common unicode emoji to the short names Linear stores.
// Linear normalizes reactions on create (e.g. "👍" is stored as "+1"), so a
// bare unicode --emoji would never match the stored form on removal without
// this. The map covers the everyday reactions; anything else still works via
// its Linear short name, which the "no match" error lists for the user.
var emojiAliases = map[string]string{
	"👍":  "+1",
	"👎":  "-1",
	"❤️": "heart",
	"❤":  "heart",
	"🎉":  "tada",
	"😄":  "smile",
	"😕":  "confused",
	"👀":  "eyes",
	"🚀":  "rocket",
	"✅":  "white_check_mark",
	"🙏":  "pray",
	"🔥":  "fire",
	"🙌":  "raised_hands",
	// Short-name synonyms for the same reactions.
	"thumbsup":   "+1",
	"thumbsdown": "-1",
}

// normalizeEmoji folds an emoji reference toward the form Linear stores it in:
// surrounding colons stripped, lower-cased, and common unicode emoji mapped to
// their Linear short names. This lets `--emoji 👍` match a stored "+1".
func normalizeEmoji(s string) string {
	s = strings.ToLower(strings.Trim(s, ":"))
	if alias, ok := emojiAliases[s]; ok {
		return alias
	}
	return s
}

// findReactionID returns the id of the reaction matching both the emoji and
// the given user. Removing a reaction targets "my" reaction specifically:
// emoji alone is ambiguous once several users have reacted the same way, so a
// match requires the owning user too. Emoji comparison is normalized so a
// unicode emoji matches Linear's stored short name.
//
// When nothing matches, the error lists the user's own reactions on the
// entity (in their stored form), so the caller knows exactly what to pass.
func findReactionID(reactions []reactionInfo, emoji, userID string) (string, error) {
	want := normalizeEmoji(emoji)
	var mine []string
	for _, r := range reactions {
		if r.userID != userID {
			continue
		}
		if normalizeEmoji(r.emoji) == want {
			return r.id, nil
		}
		mine = append(mine, r.emoji)
	}

	if len(mine) == 0 {
		return "", fmt.Errorf("no %q reaction by you found to remove", emoji)
	}
	sort.Strings(mine)
	return "", fmt.Errorf("no %q reaction by you found to remove; your reactions here: %s", emoji, strings.Join(mine, ", "))
}

// addReactionFlags registers the shared --emoji/--remove flags on a react
// command. Both `issue react` and `comment react` use the same shape.
func addReactionFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("emoji", "e", "", "Emoji to react with (required)")
	cmd.Flags().Bool("remove", false, "Remove your reaction with this emoji instead of adding it")
}

// runReaction is the shared add/remove flow for issue and comment reactions.
// kind is "issue" or "comment"; ref is the issue identifier or comment ID as
// the user supplied it (Linear's reactionCreate accepts an issue identifier
// directly for issueId).
func runReaction(cmd *cobra.Command, kind, ref string) error {
	plaintext := viper.GetBool("plaintext")
	jsonOut := viper.GetBool("json")

	emoji, _ := cmd.Flags().GetString("emoji")
	if emoji == "" {
		return errors.New("Emoji is required (--emoji)")
	}
	remove, _ := cmd.Flags().GetBool("remove")

	client, err := newGraphQLClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	if remove {
		return removeReaction(ctx, client, kind, ref, emoji, plaintext, jsonOut)
	}
	return addReaction(ctx, client, kind, ref, emoji, plaintext, jsonOut)
}

// addReaction creates a reaction on the given issue or comment.
func addReaction(ctx context.Context, client graphql.Client, kind, ref, emoji string, plaintext, jsonOut bool) error {
	input := &api.ReactionCreateInput{Emoji: emoji}
	switch kind {
	case "issue":
		input.IssueId = &ref
	case "comment":
		input.CommentId = &ref
	}

	resp, err := api.ReactionCreate(ctx, client, input)
	if err != nil {
		return fmt.Errorf("Failed to add reaction: %w", err)
	}
	if resp.ReactionCreate == nil || !resp.ReactionCreate.Success {
		return errors.New("Failed to add reaction")
	}

	if jsonOut {
		output.JSON(resp.ReactionCreate.Reaction)
		return nil
	}
	output.Success(fmt.Sprintf("Added %s reaction to %s %s", emoji, kind, ref), plaintext, jsonOut)
	return nil
}

// removeReaction deletes the current user's reaction with the given emoji from
// the issue or comment. It resolves the viewer, fetches the parent's
// reactions, and matches on emoji + owner.
func removeReaction(ctx context.Context, client graphql.Client, kind, ref, emoji string, plaintext, jsonOut bool) error {
	viewer, err := api.GetViewer(ctx, client)
	if err != nil {
		return fmt.Errorf("Failed to resolve current user: %w", err)
	}
	viewerID := viewer.Viewer.UserDetailFields.Id

	reactions, err := fetchReactions(ctx, client, kind, ref)
	if err != nil {
		return err
	}

	reactionID, err := findReactionID(reactions, emoji, viewerID)
	if err != nil {
		return err
	}

	resp, err := api.ReactionDelete(ctx, client, reactionID)
	if err != nil {
		return fmt.Errorf("Failed to remove reaction: %w", err)
	}
	if resp.ReactionDelete == nil || !resp.ReactionDelete.Success {
		return errors.New("Failed to remove reaction")
	}

	if jsonOut {
		output.JSON(map[string]interface{}{"success": true, "id": reactionID})
		return nil
	}
	output.Success(fmt.Sprintf("Removed %s reaction from %s %s", emoji, kind, ref), plaintext, jsonOut)
	return nil
}

// fetchReactions returns the current reactions on an issue or comment,
// normalized to []reactionInfo for matching.
func fetchReactions(ctx context.Context, client graphql.Client, kind, ref string) ([]reactionInfo, error) {
	switch kind {
	case "issue":
		resp, err := api.GetIssueReactions(ctx, client, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to load reactions: %w", err)
		}
		if resp.Issue == nil {
			return nil, fmt.Errorf("Issue not found: %s", ref)
		}
		out := make([]reactionInfo, 0, len(resp.Issue.Reactions))
		for _, r := range resp.Issue.Reactions {
			userID := ""
			if r.User != nil {
				userID = r.User.Id
			}
			out = append(out, reactionInfo{id: r.Id, emoji: r.Emoji, userID: userID})
		}
		return out, nil
	case "comment":
		resp, err := api.GetCommentReactions(ctx, client, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to load reactions: %w", err)
		}
		if resp.Comment == nil {
			return nil, fmt.Errorf("Comment not found: %s", ref)
		}
		out := make([]reactionInfo, 0, len(resp.Comment.Reactions))
		for _, r := range resp.Comment.Reactions {
			userID := ""
			if r.User != nil {
				userID = r.User.Id
			}
			out = append(out, reactionInfo{id: r.Id, emoji: r.Emoji, userID: userID})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown reaction target: %s", kind)
	}
}
