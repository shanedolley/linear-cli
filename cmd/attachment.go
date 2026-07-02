package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

var attachmentListCmd = &cobra.Command{
	Use:   "list <issue-id>",
	Short: "List attachments on an issue",
	Long:  `List all attachments (both files and URLs) on a Linear issue.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := args[0]

		// Get output flags
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		// Get flags
		limit, _ := cmd.Flags().GetInt("limit")
		sortFlag, _ := cmd.Flags().GetString("sort")

		// Convert sort flag to enum
		var orderByEnum *api.PaginationOrderBy
		if sortFlag != "" {
			switch sortFlag {
			case "linear":
				// Use Linear's default sort order (nil orderBy)
				orderByEnum = nil
			case "created", "createdAt":
				val := api.PaginationOrderByCreatedat
				orderByEnum = &val
			case "updated", "updatedAt":
				val := api.PaginationOrderByUpdatedat
				orderByEnum = &val
			default:
				return fmt.Errorf("Invalid sort option: %s. Valid options are: linear, created, updated", sortFlag)
			}
		}

		// Get auth and create client
		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		// Convert limit to pointer
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		// Call API
		resp, err := api.ListAttachments(ctx, client, issueID, limitPtr, nil, orderByEnum)
		if err != nil {
			return fmt.Errorf("Failed to list attachments: %v", err)
		}

		// Check if issue exists
		if resp.Issue == nil {
			return fmt.Errorf("Issue %s not found", issueID)
		}

		attachments := resp.Issue.Attachments.Nodes

		// Render output
		if jsonOut {
			output.JSON(attachments)
			return nil
		}

		if len(attachments) == 0 {
			fmt.Println("No attachments found")
			return nil
		}

		if plaintext {
			fmt.Printf("# Attachments for %s\n\n", issueID)
			for _, att := range attachments {
				fmt.Printf("## %s\n", att.Title)
				fmt.Printf("- **ID**: %s\n", att.Id)
				if att.Subtitle != nil {
					fmt.Printf("- **Subtitle**: %s\n", *att.Subtitle)
				}
				fmt.Printf("- **URL**: %s\n", att.Url)
				fmt.Printf("- **Created**: %s\n", att.CreatedAt.Format("2006-01-02"))
				if att.Creator != nil {
					fmt.Printf("- **Creator**: %s\n", att.Creator.Name)
				}
				fmt.Println()
			}
		} else {
			// Table output
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Title", "Subtitle", "Creator", "Created"})
			table.SetBorder(false)
			table.SetAutoWrapText(false)

			for _, att := range attachments {
				subtitle := ""
				if att.Subtitle != nil {
					subtitle = *att.Subtitle
				}
				creator := ""
				if att.Creator != nil {
					creator = att.Creator.Name
				}
				table.Append([]string{
					att.Id,
					att.Title,
					subtitle,
					creator,
					att.CreatedAt.Format("2006-01-02"),
				})
			}
			table.Render()
		}
		return nil
	},
}

func init() {
	attachmentCmd.AddCommand(attachmentListCmd)
	attachmentListCmd.Flags().IntP("limit", "l", 50, "Maximum number of attachments to return")
	attachmentListCmd.Flags().StringP("sort", "o", "", "Sort order: linear (default), created, updated")
}

var attachmentCreateCmd = &cobra.Command{
	Use:   "create <issue-id>",
	Short: "Create a URL attachment on an issue",
	Long:  `Create an attachment linking to an external URL (e.g., GitHub PR, documentation).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := args[0]

		// Get output flags
		jsonOut := viper.GetBool("json")

		// Get flags
		url, _ := cmd.Flags().GetString("url")
		title, _ := cmd.Flags().GetString("title")
		subtitle, _ := cmd.Flags().GetString("subtitle")
		iconURL, _ := cmd.Flags().GetString("icon-url")
		metadataStr, _ := cmd.Flags().GetString("metadata")

		// Validate required flags
		if url == "" {
			return errors.New("--url is required")
		}
		if title == "" {
			return errors.New("--title is required")
		}

		// Parse metadata
		var metadata *map[string]interface{}
		if metadataStr != "" {
			var err error
			metadataMap, err := parseMetadata(metadataStr)
			if err != nil {
				return fmt.Errorf("Invalid metadata: %v", err)
			}
			metadata = &metadataMap
		}

		// Build input
		input := api.AttachmentCreateInput{
			IssueId: issueID,
			Title:   title,
			Url:     url,
		}
		if subtitle != "" {
			input.Subtitle = &subtitle
		}
		if iconURL != "" {
			input.IconUrl = &iconURL
		}
		if metadata != nil {
			input.Metadata = metadata
		}

		// Get auth and create client
		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		// Call API
		resp, err := api.AttachmentCreate(ctx, client, &input)
		if err != nil {
			return fmt.Errorf("Failed to create attachment: %v", err)
		}

		if !resp.AttachmentCreate.Success {
			return errors.New("Failed to create attachment")
		}

		// Render output
		if jsonOut {
			output.JSON(resp.AttachmentCreate.Attachment)
			return nil
		}

		fmt.Printf("✓ Created attachment: %s\n", resp.AttachmentCreate.Attachment.Title)
		fmt.Printf("  ID: %s\n", resp.AttachmentCreate.Attachment.Id)
		fmt.Printf("  URL: %s\n", resp.AttachmentCreate.Attachment.Url)
		return nil
	},
}

// parseMetadata parses comma-separated key=value pairs
func parseMetadata(s string) (map[string]interface{}, error) {
	metadata := make(map[string]interface{})
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("metadata must be key=value pairs separated by commas")
		}
		metadata[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return metadata, nil
}

func init() {
	attachmentCmd.AddCommand(attachmentCreateCmd)
	attachmentCreateCmd.Flags().String("url", "", "URL to attach (required)")
	attachmentCreateCmd.Flags().String("title", "", "Attachment title (required)")
	attachmentCreateCmd.Flags().String("subtitle", "", "Attachment subtitle")
	attachmentCreateCmd.Flags().String("icon-url", "", "Custom icon URL")
	attachmentCreateCmd.Flags().String("metadata", "", "Metadata as key=value pairs (comma-separated)")
	attachmentCreateCmd.MarkFlagRequired("url")
	attachmentCreateCmd.MarkFlagRequired("title")
}

// fileAttachment represents a file to be uploaded
type fileAttachment struct {
	path        string
	title       string
	subtitle    string
	iconURL     string
	metadata    map[string]interface{}
	size        int64
	contentType string
}

// validationError represents a file validation error
type validationError struct {
	filename string
	error    string
}

// uploadResult represents the result of a file upload
type uploadResult struct {
	Filename string `json:"filename"`
	Title    string `json:"title"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// validateFile checks if a file is valid for upload
func validateFile(path string) (int64, error) {
	// Check file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("file not found")
		}
		if os.IsPermission(err) {
			return 0, fmt.Errorf("permission denied")
		}
		return 0, fmt.Errorf("cannot access file: %w", err)
	}

	// Check not a directory
	if info.IsDir() {
		return 0, fmt.Errorf("is a directory, not a file")
	}

	// Check size
	size := info.Size()
	const maxSize = 50 * 1024 * 1024 // 50MB
	if size > maxSize {
		return 0, fmt.Errorf("file size %.1f MB exceeds limit of 50 MB", float64(size)/(1024*1024))
	}

	return size, nil
}

// detectContentType detects MIME type of a file
func detectContentType(path string) (string, error) {
	// Try extension-based detection first
	ext := strings.ToLower(filepath.Ext(path))
	if contentType := mime.TypeByExtension(ext); contentType != "" {
		return contentType, nil
	}

	// Fallback to content-based detection
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read first 512 bytes for detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	return http.DetectContentType(buffer[:n]), nil
}

// formatSize formats bytes as human-readable size
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// uploadFileToLinear uploads a file through Linear's file upload flow
func uploadFileToLinear(ctx context.Context, client graphql.Client, file fileAttachment, issueID string, quiet bool) error {
	// Step 1: Get pre-signed URL
	uploadResp, err := api.FileUpload(ctx, client, file.contentType, filepath.Base(file.path), int(file.size))
	if err != nil {
		return fmt.Errorf("failed to get upload URL: %w", err)
	}

	// Step 2: Upload file with progress tracking
	if err := uploadFileWithProgress(file.path, uploadResp.FileUpload.UploadFile.UploadUrl, uploadResp.FileUpload.UploadFile.Headers, file.contentType, file.size, quiet); err != nil {
		return err
	}

	// Step 3: Create attachment using asset URL
	input := api.AttachmentCreateInput{
		IssueId: issueID,
		Title:   file.title,
		Url:     uploadResp.FileUpload.UploadFile.AssetUrl,
	}
	if file.subtitle != "" {
		input.Subtitle = &file.subtitle
	}
	if file.iconURL != "" {
		input.IconUrl = &file.iconURL
	}
	if file.metadata != nil {
		input.Metadata = &file.metadata
	}

	attachResp, err := api.AttachmentCreate(ctx, client, &input)
	if err != nil {
		return fmt.Errorf("failed to create attachment: %w", err)
	}

	if !attachResp.AttachmentCreate.Success {
		return fmt.Errorf("attachment creation failed")
	}

	return nil
}

// uploadFileWithProgress uploads file to URL with progress bar and retry logic
func uploadFileWithProgress(filePath, uploadURL string, headers []*api.FileUploadFileUploadUploadPayloadUploadFileHeadersUploadFileHeader, contentType string, fileSize int64, quiet bool) error {
	const maxRetries = 3
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 && !quiet {
			fmt.Printf("✗ Network error, retrying... (attempt %d/%d)\n", attempt, maxRetries)
			time.Sleep(backoff[attempt-1])
		}

		// Wrap each retry attempt in anonymous function so defer runs per iteration
		err := func() error {
			// Open file
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			// Build request with appropriate reader
			var req *http.Request
			if quiet {
				// In quiet mode, use the file directly without progress bar
				req, err = http.NewRequest("PUT", uploadURL, file)
			} else {
				// Create progress bar and wrap file reader with progress tracking
				bar := progressbar.DefaultBytes(
					fileSize,
					fmt.Sprintf("Uploading %s (%s)...", filepath.Base(filePath), formatSize(fileSize)),
				)
				reader := progressbar.NewReader(file, bar)
				req, err = http.NewRequest("PUT", uploadURL, &reader)
			}
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			// Set Content-Length (required for pre-signed URL uploads)
			req.ContentLength = fileSize

			// Set Content-Type to match what we told Linear in FileUpload mutation
			req.Header.Set("Content-Type", contentType)

			// Add headers from Linear (Content-Disposition, content-length-range, etc)
			for _, h := range headers {
				req.Header.Set(h.Key, h.Value)
			}

			// Execute upload
			client := &http.Client{Timeout: 5 * time.Minute}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("upload failed: %w", err)
			}
			defer resp.Body.Close()

			// Read response body to allow connection reuse
			_, _ = io.Copy(io.Discard, resp.Body)

			// Check status
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}

			// 4xx errors should not be retried
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return fmt.Errorf("upload failed with status %d (non-retryable)", resp.StatusCode)
			}

			// 5xx errors can be retried
			return fmt.Errorf("upload failed with status %d", resp.StatusCode)
		}()

		// Success case
		if err == nil {
			return nil
		}

		// Save error for potential retry
		lastErr = err

		// Check if error is non-retryable (4xx status)
		if strings.Contains(err.Error(), "non-retryable") {
			return lastErr
		}
	}

	return fmt.Errorf("upload failed after %d retries: %w", maxRetries, lastErr)
}

var attachmentUploadCmd = &cobra.Command{
	Use:   "upload <issue-id>",
	Short: "Upload files as attachments to an issue",
	Long: `Upload one or more files to Linear's storage and attach them to an issue.

Each --file flag starts a new attachment. Required flags for each:
  --file: Path to file
  --title: Attachment title

Optional per-file flags:
  --subtitle: Attachment subtitle
  --icon-url: Custom icon URL
  --metadata: key=value pairs (comma-separated)

Example:
  lincli attachment upload LIN-123 \
    --file report.pdf --title "Q4 Report" --subtitle "Draft" \
    --file screenshot.png --title "Bug Screenshot"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := args[0]

		// Get output flags
		jsonOut := viper.GetBool("json")

		// Parse file attachments from flags
		files, err := parseFileFlags(cmd)
		if err != nil {
			return err
		}

		if len(files) == 0 {
			return errors.New("At least one --file and --title pair is required")
		}

		// Validate all files first
		if !jsonOut {
			fmt.Println("Validating files...")
		}
		var validationErrors []validationError
		for i := range files {
			size, err := validateFile(files[i].path)
			if err != nil {
				validationErrors = append(validationErrors, validationError{
					filename: filepath.Base(files[i].path),
					error:    err.Error(),
				})
				continue
			}
			files[i].size = size

			// Detect content type
			contentType, err := detectContentType(files[i].path)
			if err != nil {
				validationErrors = append(validationErrors, validationError{
					filename: filepath.Base(files[i].path),
					error:    fmt.Sprintf("failed to detect content type: %v", err),
				})
				continue
			}
			files[i].contentType = contentType

			if !jsonOut {
				fmt.Printf("✓ %s (%s) - OK\n", filepath.Base(files[i].path), formatSize(size))
			}
		}

		// Stop if validation failed
		if len(validationErrors) > 0 {
			if jsonOut {
				// In JSON mode, output validation errors as JSON
				errorList := make([]map[string]string, len(validationErrors))
				for i, ve := range validationErrors {
					errorList[i] = map[string]string{
						"filename": ve.filename,
						"error":    ve.error,
					}
				}
				output.JSON(map[string]interface{}{
					"error":             "Validation failed",
					"validation_errors": errorList,
				})
			} else {
				fmt.Println("\nError: Validation failed:")
				for _, ve := range validationErrors {
					fmt.Printf("  - %s: %s\n", ve.filename, ve.error)
				}
			}
			return errSilent
		}

		// Get auth and create client
		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		// Upload files and collect results
		if !jsonOut {
			fmt.Println()
		}
		var results []uploadResult
		var succeeded, failed int

		for _, file := range files {
			err := uploadFileToLinear(ctx, client, file, issueID, jsonOut)
			result := uploadResult{
				Filename: filepath.Base(file.path),
				Title:    file.title,
				Success:  err == nil,
			}
			if err != nil {
				result.Error = err.Error()
				failed++
				if !jsonOut {
					fmt.Printf("✗ Failed to attach %s: %v\n", filepath.Base(file.path), err)
				}
			} else {
				succeeded++
				if !jsonOut {
					fmt.Printf("✓ Attached %s to %s\n", filepath.Base(file.path), issueID)
				}
			}
			results = append(results, result)
			if !jsonOut {
				fmt.Println()
			}
		}

		// Output results
		if jsonOut {
			output.JSON(map[string]interface{}{
				"succeeded": succeeded,
				"failed":    failed,
				"results":   results,
			})
			if failed > 0 {
				return errSilent
			}
		} else {
			// Summary
			fmt.Printf("Summary: %d succeeded, %d failed\n", succeeded, failed)
			if failed > 0 {
				fmt.Println("Failed uploads:")
				for _, r := range results {
					if !r.Success {
						fmt.Printf("  - %s: %s\n", r.Filename, r.Error)
					}
				}
				return errSilent
			}
		}
		return nil
	},
}

// parseFileFlags parses --file, --title, --subtitle, etc. into fileAttachment structs
func parseFileFlags(cmd *cobra.Command) ([]fileAttachment, error) {
	// Get all flag values
	files, _ := cmd.Flags().GetStringArray("file")
	titles, _ := cmd.Flags().GetStringArray("title")
	subtitles, _ := cmd.Flags().GetStringArray("subtitle")
	iconURLs, _ := cmd.Flags().GetStringArray("icon-url")
	metadatas, _ := cmd.Flags().GetStringArray("metadata")

	// Validate counts
	if len(files) != len(titles) {
		return nil, fmt.Errorf("each --file must have a corresponding --title")
	}

	// Build attachments
	var attachments []fileAttachment
	for i := range files {
		// Validate title is not empty
		if titles[i] == "" {
			return nil, fmt.Errorf("title for file %s cannot be empty", files[i])
		}

		att := fileAttachment{
			path:  files[i],
			title: titles[i],
		}

		if i < len(subtitles) && subtitles[i] != "" {
			att.subtitle = subtitles[i]
		}
		if i < len(iconURLs) && iconURLs[i] != "" {
			att.iconURL = iconURLs[i]
		}
		if i < len(metadatas) && metadatas[i] != "" {
			metadata, err := parseMetadata(metadatas[i])
			if err != nil {
				return nil, fmt.Errorf("invalid metadata for file %s: %w", files[i], err)
			}
			att.metadata = metadata
		}

		attachments = append(attachments, att)
	}

	return attachments, nil
}

func init() {
	attachmentCmd.AddCommand(attachmentUploadCmd)
	attachmentUploadCmd.Flags().StringArray("file", []string{}, "Path to file to upload (required)")
	attachmentUploadCmd.Flags().StringArray("title", []string{}, "Attachment title (required for each file)")
	attachmentUploadCmd.Flags().StringArray("subtitle", []string{}, "Attachment subtitle")
	attachmentUploadCmd.Flags().StringArray("icon-url", []string{}, "Custom icon URL")
	attachmentUploadCmd.Flags().StringArray("metadata", []string{}, "Metadata as key=value pairs")
}

var attachmentUpdateCmd = &cobra.Command{
	Use:   "update <attachment-id>",
	Short: "Update an attachment's metadata",
	Long: `Update an attachment's title, subtitle, icon, or metadata.

Note: Linear's API does not support changing an attachment's URL after creation.
To change a file or URL, delete the old attachment and create a new one.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		attachmentID := args[0]

		// Get output flags
		jsonOut := viper.GetBool("json")

		// Get flags
		title, _ := cmd.Flags().GetString("title")
		subtitle, _ := cmd.Flags().GetString("subtitle")
		iconURL, _ := cmd.Flags().GetString("icon-url")
		metadataStr, _ := cmd.Flags().GetString("metadata")

		// Check if at least one field other than title is being changed
		// (title alone is valid, but we require at least one field to change)
		if !cmd.Flags().Changed("title") && !cmd.Flags().Changed("subtitle") &&
			!cmd.Flags().Changed("icon-url") && !cmd.Flags().Changed("metadata") {
			return errors.New("No fields to update (specify --title, --subtitle, --icon-url, or --metadata)")
		}

		// Get auth and create client
		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		// Build update input
		input := api.AttachmentUpdateInput{
			Title: title, // Required field
		}
		if cmd.Flags().Changed("subtitle") {
			input.Subtitle = &subtitle
		}
		if cmd.Flags().Changed("icon-url") {
			input.IconUrl = &iconURL
		}
		if cmd.Flags().Changed("metadata") {
			metadata, err := parseMetadata(metadataStr)
			if err != nil {
				return fmt.Errorf("Invalid metadata: %v", err)
			}
			input.Metadata = &metadata
		}

		// Call API
		resp, err := api.AttachmentUpdate(ctx, client, attachmentID, &input)
		if err != nil {
			return fmt.Errorf("Failed to update attachment: %v", err)
		}

		if !resp.AttachmentUpdate.Success {
			return errors.New("Failed to update attachment")
		}

		// Render output
		if jsonOut {
			output.JSON(resp.AttachmentUpdate.Attachment)
			return nil
		}

		fmt.Printf("✓ Updated attachment: %s\n", resp.AttachmentUpdate.Attachment.Title)
		return nil
	},
}

func init() {
	attachmentCmd.AddCommand(attachmentUpdateCmd)
	attachmentUpdateCmd.Flags().String("title", "", "Attachment title (required)")
	attachmentUpdateCmd.Flags().String("subtitle", "", "New subtitle")
	attachmentUpdateCmd.Flags().String("icon-url", "", "New icon URL")
	attachmentUpdateCmd.Flags().String("metadata", "", "New metadata as key=value pairs")
	attachmentUpdateCmd.MarkFlagRequired("title")
}

var attachmentDeleteCmd = &cobra.Command{
	Use:   "delete <attachment-id>",
	Short: "Delete an attachment",
	Long:  `Delete an attachment from an issue. This action cannot be undone.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		attachmentID := args[0]

		// Get output flags
		jsonOut := viper.GetBool("json")

		// Get auth and create client
		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		// Call API
		resp, err := api.AttachmentDelete(ctx, client, attachmentID)
		if err != nil {
			return fmt.Errorf("Failed to delete attachment: %v", err)
		}

		if !resp.AttachmentDelete.Success {
			return errors.New("Failed to delete attachment")
		}

		// Render output
		if jsonOut {
			output.JSON(map[string]bool{"success": true})
			return nil
		}

		fmt.Printf("✓ Deleted attachment %s\n", attachmentID)
		return nil
	},
}

func init() {
	attachmentCmd.AddCommand(attachmentDeleteCmd)
}

// --- Typed link attachments and Slack sync (Tier 4c) ---

// attachmentLinkProviders lists the canonical --provider values, in the order
// shown to the user. Each maps to a typed attachmentLink* mutation that takes
// only (issueId, url).
var attachmentLinkProviders = []string{"url", "slack", "github-issue", "github-pr", "salesforce"}

// attachmentProviderAliases maps convenient shorthands to a canonical provider.
var attachmentProviderAliases = map[string]string{
	"github":   "github-issue",
	"gh":       "github-issue",
	"gh-issue": "github-issue",
	"gh-pr":    "github-pr",
	"pr":       "github-pr",
	"sf":       "salesforce",
}

// normalizeAttachmentProvider maps a --provider value (canonical name or alias,
// case-insensitively) to its canonical form. An unknown value returns an error
// listing the valid providers.
func normalizeAttachmentProvider(provider string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	if canonical, ok := attachmentProviderAliases[p]; ok {
		p = canonical
	}
	for _, valid := range attachmentLinkProviders {
		if p == valid {
			return p, nil
		}
	}
	return "", fmt.Errorf("unknown provider %q: valid providers are %s", provider, strings.Join(attachmentLinkProviders, ", "))
}

var attachmentLinkCmd = &cobra.Command{
	Use:   "link <issue-id> <url>",
	Short: "Link a URL to an issue as a typed attachment",
	Long: `Link a URL to an issue, dispatching to a provider-typed attachment mutation.

The default provider 'url' auto-detects recognized integration URLs (Zendesk,
Jira, GitLab, GitHub, Slack, ...) and builds the matching rich attachment, so it
covers most cases. Use --provider to force a specific type.

Providers: url (default), slack, github-issue, github-pr, salesforce.

Examples:
  lincli attachment link ENG-123 https://github.com/org/repo/pull/42 --provider github-pr
  lincli attachment link ENG-123 https://acme.slack.com/archives/C0/p1 --provider slack --sync-to-comment-thread`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := args[0]
		url := args[1]

		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		providerRaw, _ := cmd.Flags().GetString("provider")
		provider, err := normalizeAttachmentProvider(providerRaw)
		if err != nil {
			return err
		}

		var titlePtr *string
		if cmd.Flags().Changed("title") {
			title, _ := cmd.Flags().GetString("title")
			titlePtr = &title
		}

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		// Each provider maps to a typed mutation returning an AttachmentPayload.
		// The payload's attachment field is a pointer (genqlient optional:pointer),
		// so check success and guard the attachment before dereferencing, matching
		// the check-before-use ordering in the other write commands (see
		// attachmentCreateCmd, cmd/customer.go).
		var (
			success                 bool
			attID, attTitle, attURL string
			attCreated              time.Time
		)
		switch provider {
		case "url":
			resp, err := api.AttachmentLinkURL(ctx, client, issueID, url, titlePtr)
			if err != nil {
				return fmt.Errorf("Failed to link attachment: %v", err)
			}
			if resp.AttachmentLinkURL != nil && resp.AttachmentLinkURL.Success {
				if a := resp.AttachmentLinkURL.Attachment; a != nil {
					success = true
					attID, attTitle, attURL, attCreated = a.Id, a.Title, a.Url, a.CreatedAt
				}
			}
		case "slack":
			var syncPtr *bool
			if cmd.Flags().Changed("sync-to-comment-thread") {
				sync, _ := cmd.Flags().GetBool("sync-to-comment-thread")
				syncPtr = &sync
			}
			resp, err := api.AttachmentLinkSlack(ctx, client, issueID, url, titlePtr, syncPtr)
			if err != nil {
				return fmt.Errorf("Failed to link attachment: %v", err)
			}
			if resp.AttachmentLinkSlack != nil && resp.AttachmentLinkSlack.Success {
				if a := resp.AttachmentLinkSlack.Attachment; a != nil {
					success = true
					attID, attTitle, attURL, attCreated = a.Id, a.Title, a.Url, a.CreatedAt
				}
			}
		case "github-issue":
			resp, err := api.AttachmentLinkGitHubIssue(ctx, client, issueID, url, titlePtr)
			if err != nil {
				return fmt.Errorf("Failed to link attachment: %v", err)
			}
			if resp.AttachmentLinkGitHubIssue != nil && resp.AttachmentLinkGitHubIssue.Success {
				if a := resp.AttachmentLinkGitHubIssue.Attachment; a != nil {
					success = true
					attID, attTitle, attURL, attCreated = a.Id, a.Title, a.Url, a.CreatedAt
				}
			}
		case "github-pr":
			resp, err := api.AttachmentLinkGitHubPR(ctx, client, issueID, url, titlePtr)
			if err != nil {
				return fmt.Errorf("Failed to link attachment: %v", err)
			}
			if resp.AttachmentLinkGitHubPR != nil && resp.AttachmentLinkGitHubPR.Success {
				if a := resp.AttachmentLinkGitHubPR.Attachment; a != nil {
					success = true
					attID, attTitle, attURL, attCreated = a.Id, a.Title, a.Url, a.CreatedAt
				}
			}
		case "salesforce":
			resp, err := api.AttachmentLinkSalesforce(ctx, client, issueID, url, titlePtr)
			if err != nil {
				return fmt.Errorf("Failed to link attachment: %v", err)
			}
			if resp.AttachmentLinkSalesforce != nil && resp.AttachmentLinkSalesforce.Success {
				if a := resp.AttachmentLinkSalesforce.Attachment; a != nil {
					success = true
					attID, attTitle, attURL, attCreated = a.Id, a.Title, a.Url, a.CreatedAt
				}
			}
		}

		if !success {
			return errors.New("Failed to link attachment")
		}

		if jsonOut {
			output.JSON(map[string]interface{}{"id": attID, "title": attTitle, "url": attURL, "createdAt": attCreated})
			return nil
		}
		output.Success(fmt.Sprintf("Linked %s attachment to %s", provider, issueID), plaintext, jsonOut)
		fmt.Printf("  ID:    %s\n", attID)
		if attTitle != "" {
			fmt.Printf("  Title: %s\n", attTitle)
		}
		fmt.Printf("  URL:   %s\n", attURL)
		return nil
	},
}

var attachmentSyncToSlackCmd = &cobra.Command{
	Use:   "sync-to-slack <attachment-id>",
	Short: "Sync a Slack attachment's thread to the issue comment thread",
	Long:  `Begin syncing an existing Slack message attachment's thread with the issue's comment thread.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		client, err := newGraphQLClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		resp, err := api.AttachmentSyncToSlack(ctx, client, args[0])
		if err != nil {
			return fmt.Errorf("Failed to sync attachment to Slack: %v", err)
		}
		if resp.AttachmentSyncToSlack == nil || !resp.AttachmentSyncToSlack.Success {
			return errors.New("Failed to sync attachment to Slack")
		}

		if jsonOut {
			output.JSON(resp.AttachmentSyncToSlack.Attachment)
		} else {
			output.Success(fmt.Sprintf("Syncing attachment %s to Slack", args[0]), plaintext, jsonOut)
		}
		return nil
	},
}

func init() {
	attachmentCmd.AddCommand(attachmentLinkCmd)
	attachmentCmd.AddCommand(attachmentSyncToSlackCmd)

	attachmentLinkCmd.Flags().StringP("provider", "P", "url", "Attachment provider: url, slack, github-issue, github-pr, salesforce")
	attachmentLinkCmd.Flags().String("title", "", "Attachment title")
	attachmentLinkCmd.Flags().Bool("sync-to-comment-thread", false, "For --provider slack: sync the Slack thread to the issue comment thread")
}
