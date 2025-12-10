package cmd

import (
	"context"
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
	"github.com/shanedolley/lincli/pkg/auth"
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
	Run: func(cmd *cobra.Command, args []string) {
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
				output.Error(fmt.Sprintf("Invalid sort option: %s. Valid options are: linear, created, updated", sortFlag), plaintext, jsonOut)
				os.Exit(1)
			}
		}

		// Get auth and create client
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		// Convert limit to pointer
		var limitPtr *int
		if limit > 0 {
			limitPtr = &limit
		}

		// Call API
		resp, err := api.ListAttachments(ctx, client, issueID, limitPtr, nil, orderByEnum)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to list attachments: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		// Check if issue exists
		if resp.Issue == nil {
			output.Error(fmt.Sprintf("Issue %s not found", issueID), plaintext, jsonOut)
			os.Exit(1)
		}

		attachments := resp.Issue.Attachments.Nodes

		// Render output
		if jsonOut {
			output.JSON(attachments)
			return
		}

		if len(attachments) == 0 {
			fmt.Println("No attachments found")
			return
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
	Run: func(cmd *cobra.Command, args []string) {
		issueID := args[0]

		// Get output flags
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		// Get flags
		url, _ := cmd.Flags().GetString("url")
		title, _ := cmd.Flags().GetString("title")
		subtitle, _ := cmd.Flags().GetString("subtitle")
		iconURL, _ := cmd.Flags().GetString("icon-url")
		metadataStr, _ := cmd.Flags().GetString("metadata")

		// Validate required flags
		if url == "" {
			output.Error("--url is required", plaintext, jsonOut)
			os.Exit(1)
		}
		if title == "" {
			output.Error("--title is required", plaintext, jsonOut)
			os.Exit(1)
		}

		// Parse metadata
		var metadata *map[string]interface{}
		if metadataStr != "" {
			var err error
			metadataMap, err := parseMetadata(metadataStr)
			if err != nil {
				output.Error(fmt.Sprintf("Invalid metadata: %v", err), plaintext, jsonOut)
				os.Exit(1)
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
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		// Call API
		resp, err := api.AttachmentCreate(ctx, client, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to create attachment: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if !resp.AttachmentCreate.Success {
			output.Error("Failed to create attachment", plaintext, jsonOut)
			os.Exit(1)
		}

		// Render output
		if jsonOut {
			output.JSON(resp.AttachmentCreate.Attachment)
			return
		}

		fmt.Printf("✓ Created attachment: %s\n", resp.AttachmentCreate.Attachment.Title)
		fmt.Printf("  ID: %s\n", resp.AttachmentCreate.Attachment.Id)
		fmt.Printf("  URL: %s\n", resp.AttachmentCreate.Attachment.Url)
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
	if err := uploadFileWithProgress(file.path, uploadResp.FileUpload.UploadFile.UploadUrl, uploadResp.FileUpload.UploadFile.Headers, file.size, quiet); err != nil {
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
func uploadFileWithProgress(filePath, uploadURL string, headers []*api.FileUploadFileUploadUploadPayloadUploadFileHeadersUploadFileHeader, fileSize int64, quiet bool) error {
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

			// Add headers from Linear
			for _, h := range headers {
				req.Header.Set(h.Key, h.Value)
			}
			// Add required headers
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("Cache-Control", "public, max-age=31536000")

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
	Run: func(cmd *cobra.Command, args []string) {
		issueID := args[0]

		// Get output flags
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		// Parse file attachments from flags
		files, err := parseFileFlags(cmd)
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		if len(files) == 0 {
			output.Error("At least one --file and --title pair is required", plaintext, jsonOut)
			os.Exit(1)
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
			os.Exit(1)
		}

		// Get auth and create client
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
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
				os.Exit(1)
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
				os.Exit(1)
			}
		}
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
	Run: func(cmd *cobra.Command, args []string) {
		attachmentID := args[0]

		// Get output flags
		plaintext := viper.GetBool("plaintext")
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
			output.Error("No fields to update (specify --title, --subtitle, --icon-url, or --metadata)", plaintext, jsonOut)
			os.Exit(1)
		}

		// Get auth and create client
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
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
				output.Error(fmt.Sprintf("Invalid metadata: %v", err), plaintext, jsonOut)
				os.Exit(1)
			}
			input.Metadata = &metadata
		}

		// Call API
		resp, err := api.AttachmentUpdate(ctx, client, attachmentID, &input)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to update attachment: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if !resp.AttachmentUpdate.Success {
			output.Error("Failed to update attachment", plaintext, jsonOut)
			os.Exit(1)
		}

		// Render output
		if jsonOut {
			output.JSON(resp.AttachmentUpdate.Attachment)
			return
		}

		fmt.Printf("✓ Updated attachment: %s\n", resp.AttachmentUpdate.Attachment.Title)
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
	Run: func(cmd *cobra.Command, args []string) {
		attachmentID := args[0]

		// Get output flags
		plaintext := viper.GetBool("plaintext")
		jsonOut := viper.GetBool("json")

		// Get auth and create client
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(err.Error(), plaintext, jsonOut)
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		// Call API
		resp, err := api.AttachmentDelete(ctx, client, attachmentID)
		if err != nil {
			output.Error(fmt.Sprintf("Failed to delete attachment: %v", err), plaintext, jsonOut)
			os.Exit(1)
		}

		if !resp.AttachmentDelete.Success {
			output.Error("Failed to delete attachment", plaintext, jsonOut)
			os.Exit(1)
		}

		// Render output
		if jsonOut {
			output.JSON(map[string]bool{"success": true})
			return
		}

		fmt.Printf("✓ Deleted attachment %s\n", attachmentID)
	},
}

func init() {
	attachmentCmd.AddCommand(attachmentDeleteCmd)
}
