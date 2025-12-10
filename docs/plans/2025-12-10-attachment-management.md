# Attachment Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add comprehensive attachment management to lincli, supporting file uploads to Linear's storage and full CRUD operations for both file and URL attachments.

**Architecture:** Five commands under `attachment` subcommand following existing lincli patterns. File uploads use two-step process: GraphQL mutation for pre-signed URL, HTTP PUT for actual upload, then attachmentCreate to link to issue. All operations use genqlient-generated types for type safety.

**Tech Stack:** Go 1.24.5, genqlient for GraphQL code generation, progressbar/v3 for upload progress, standard library for file operations and HTTP

---

### Task 1: Add GraphQL Operations

**Files:**
- Create: `pkg/api/operations/attachments.graphql`

**Step 1: Create attachments GraphQL operations file**

```graphql
# Step 1 of file upload: Get pre-signed URL
mutation FileUpload($contentType: String!, $filename: String!, $size: Int!) {
  fileUpload(contentType: $contentType, filename: $filename, size: $size) {
    uploadUrl
    assetUrl
    headers {
      key
      value
    }
  }
}

# Step 2 of file upload & URL attachments: Create attachment
mutation AttachmentCreate($input: AttachmentCreateInput!) {
  attachmentCreate(input: $input) {
    success
    attachment {
      id
      title
      url
      createdAt
    }
  }
}

# List attachments on an issue
query ListAttachments($issueId: ID!, $first: Int, $after: String, $orderBy: PaginationOrderBy) {
  issue(id: $issueId) {
    id
    attachments(first: $first, after: $after, orderBy: $orderBy) {
      nodes {
        id
        title
        subtitle
        url
        iconUrl
        metadata
        createdAt
        updatedAt
        creator {
          email
          name
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}

# Update attachment metadata
mutation AttachmentUpdate($id: ID!, $input: AttachmentUpdateInput!) {
  attachmentUpdate(id: $id, input: $input) {
    success
    attachment {
      id
      title
      subtitle
      url
    }
  }
}

# Delete attachment
mutation AttachmentDelete($id: ID!) {
  attachmentDelete(id: $id) {
    success
  }
}
```

**Step 2: Generate genqlient code**

Run: `go generate ./pkg/api`
Expected: New functions in `pkg/api/generated.go` (FileUpload, AttachmentCreate, ListAttachments, AttachmentUpdate, AttachmentDelete)

**Step 3: Verify code compiles**

Run: `go build ./pkg/api`
Expected: Clean build, no errors

**Step 4: Commit**

```bash
git add pkg/api/operations/attachments.graphql pkg/api/generated.go
git commit -m "feat: add GraphQL operations for attachment management

- Add FileUpload mutation for pre-signed URLs
- Add AttachmentCreate/Update/Delete mutations
- Add ListAttachments query with pagination
- Generate type-safe code with genqlient"
```

---

### Task 2: Add Progress Bar Dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Add progressbar dependency**

Run: `go get github.com/schollz/progressbar/v3@latest`
Expected: Dependency added to go.mod and go.sum

**Step 2: Verify dependency**

Run: `go mod tidy`
Expected: Clean, no changes needed

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add progressbar/v3 for file upload progress"
```

---

### Task 3: Create Attachment Command Structure

**Files:**
- Create: `cmd/attachment.go`
- Modify: `cmd/root.go` (register attachment command)

**Step 1: Create base attachment command**

In `cmd/attachment.go`:

```go
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
```

**Step 2: Register attachment command in root**

No modification needed - `init()` handles registration automatically.

**Step 3: Verify command appears**

Run: `go run main.go attachment --help`
Expected: Shows attachment command help with "Manage issue attachments" description

**Step 4: Commit**

```bash
git add cmd/attachment.go
git commit -m "feat: add attachment command base structure"
```

---

### Task 4: Implement Attachment List Command

**Files:**
- Modify: `cmd/attachment.go`

**Step 1: Add list subcommand**

Append to `cmd/attachment.go`:

```go
var attachmentListCmd = &cobra.Command{
	Use:   "list <issue-id>",
	Short: "List attachments on an issue",
	Long:  `List all attachments (both files and URLs) on a Linear issue.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		issueID := args[0]

		// Get flags
		limit, _ := cmd.Flags().GetInt("limit")
		sortFlag, _ := cmd.Flags().GetString("sort")

		// Convert sort flag to enum
		var orderByEnum *api.PaginationOrderBy
		if sortFlag != "" {
			orderBy := parseSortOrder(sortFlag)
			orderByEnum = &orderBy
		}

		// Get auth and create client
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(jsonOutput, plaintextOutput, err.Error())
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
			output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("Failed to list attachments: %v", err))
			os.Exit(1)
		}

		// Check if issue exists
		if resp.Issue == nil {
			output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("Issue %s not found", issueID))
			os.Exit(1)
		}

		attachments := resp.Issue.Attachments.Nodes

		// Render output
		if jsonOutput {
			output.JSON(attachments)
			return
		}

		if len(attachments) == 0 {
			fmt.Println("No attachments found")
			return
		}

		if plaintextOutput {
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
	attachmentListCmd.Flags().StringP("sort", "o", "created", "Sort order: created, updated")
}
```

**Step 2: Add required imports**

Add to top of `cmd/attachment.go`:

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
)
```

**Step 3: Test list command**

Run: `go run main.go attachment list --help`
Expected: Shows help for list command with flags

**Step 4: Build to verify compilation**

Run: `make build`
Expected: Clean build

**Step 5: Commit**

```bash
git add cmd/attachment.go
git commit -m "feat: implement attachment list command

- List attachments with pagination and sorting
- Support table, plaintext, and JSON output
- Handle empty attachment lists gracefully"
```

---

### Task 5: Implement Attachment Create (URL) Command

**Files:**
- Modify: `cmd/attachment.go`

**Step 1: Add create subcommand**

Append to `cmd/attachment.go`:

```go
var attachmentCreateCmd = &cobra.Command{
	Use:   "create <issue-id>",
	Short: "Create a URL attachment on an issue",
	Long:  `Create an attachment linking to an external URL (e.g., GitHub PR, documentation).`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		issueID := args[0]

		// Get flags
		url, _ := cmd.Flags().GetString("url")
		title, _ := cmd.Flags().GetString("title")
		subtitle, _ := cmd.Flags().GetString("subtitle")
		iconURL, _ := cmd.Flags().GetString("icon-url")
		metadataStr, _ := cmd.Flags().GetString("metadata")

		// Validate required flags
		if url == "" {
			output.Error(jsonOutput, plaintextOutput, "--url is required")
			os.Exit(1)
		}
		if title == "" {
			output.Error(jsonOutput, plaintextOutput, "--title is required")
			os.Exit(1)
		}

		// Parse metadata
		var metadata map[string]interface{}
		if metadataStr != "" {
			var err error
			metadata, err = parseMetadata(metadataStr)
			if err != nil {
				output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("Invalid metadata: %v", err))
				os.Exit(1)
			}
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
			output.Error(jsonOutput, plaintextOutput, err.Error())
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		// Call API
		resp, err := api.AttachmentCreate(ctx, client, input)
		if err != nil {
			output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("Failed to create attachment: %v", err))
			os.Exit(1)
		}

		if !resp.AttachmentCreate.Success {
			output.Error(jsonOutput, plaintextOutput, "Failed to create attachment")
			os.Exit(1)
		}

		// Render output
		if jsonOutput {
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
```

**Step 2: Add strings import**

Update imports in `cmd/attachment.go`:

```go
import (
	"context"
	"fmt"
	"os"
	"strings"  // Add this

	// ... rest of imports
)
```

**Step 3: Test create command**

Run: `go run main.go attachment create --help`
Expected: Shows help with required flags marked

**Step 4: Build to verify**

Run: `make build`
Expected: Clean build

**Step 5: Commit**

```bash
git add cmd/attachment.go
git commit -m "feat: implement attachment create command for URLs

- Create URL attachments with full metadata support
- Parse comma-separated metadata key=value pairs
- Validate required flags (url, title)
- Support optional subtitle, icon-url, metadata"
```

---

### Task 6: Add File Validation Helpers

**Files:**
- Modify: `cmd/attachment.go`

**Step 1: Add file validation types and functions**

Append to `cmd/attachment.go`:

```go
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

// validateFile checks if a file is valid for upload
func validateFile(path string) (int64, error) {
	// Check file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("file not found")
		}
		return 0, fmt.Errorf("permission denied")
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
```

**Step 2: Add required imports for validation**

Update imports:

```go
import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	// ... rest of imports
)
```

**Step 3: Build to verify**

Run: `make build`
Expected: Clean build

**Step 4: Commit**

```bash
git add cmd/attachment.go
git commit -m "feat: add file validation helpers

- Validate file exists, readable, and under 50MB
- Detect content type via extension or content sniffing
- Format file sizes in human-readable format
- Define fileAttachment and validationError types"
```

---

### Task 7: Add File Upload Helpers

**Files:**
- Modify: `cmd/attachment.go`

**Step 1: Add upload helper functions**

Append to `cmd/attachment.go`:

```go
// uploadFileToLinear uploads a file through Linear's file upload flow
func uploadFileToLinear(ctx context.Context, client graphql.Client, file fileAttachment, issueID string) error {
	// Step 1: Get pre-signed URL
	uploadResp, err := api.FileUpload(ctx, client, file.contentType, filepath.Base(file.path), int(file.size))
	if err != nil {
		return fmt.Errorf("failed to get upload URL: %w", err)
	}

	// Step 2: Upload file with progress tracking
	if err := uploadFileWithProgress(file.path, uploadResp.FileUpload.UploadUrl, uploadResp.FileUpload.Headers, file.size); err != nil {
		return err
	}

	// Step 3: Create attachment using asset URL
	input := api.AttachmentCreateInput{
		IssueId: issueID,
		Title:   file.title,
		Url:     uploadResp.FileUpload.AssetUrl,
	}
	if file.subtitle != "" {
		input.Subtitle = &file.subtitle
	}
	if file.iconURL != "" {
		input.IconUrl = &file.iconURL
	}
	if file.metadata != nil {
		input.Metadata = file.metadata
	}

	attachResp, err := api.AttachmentCreate(ctx, client, input)
	if err != nil {
		return fmt.Errorf("failed to create attachment: %w", err)
	}

	if !attachResp.AttachmentCreate.Success {
		return fmt.Errorf("attachment creation failed")
	}

	return nil
}

// uploadFileWithProgress uploads file to URL with progress bar and retry logic
func uploadFileWithProgress(filePath, uploadURL string, headers []api.FileUploadFileUploadHeaders, fileSize int64) error {
	const maxRetries = 3
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("✗ Network error, retrying... (attempt %d/%d)\n", attempt, maxRetries)
			time.Sleep(backoff[attempt-1])
		}

		// Open file
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		// Create progress bar
		bar := progressbar.DefaultBytes(
			fileSize,
			fmt.Sprintf("Uploading %s (%s)...", filepath.Base(filePath), formatSize(fileSize)),
		)

		// Wrap file reader with progress tracking
		reader := progressbar.NewReader(file, bar)

		// Build request
		req, err := http.NewRequest("PUT", uploadURL, &reader)
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
			lastErr = fmt.Errorf("upload failed: %w", err)
			file.Close()
			continue
		}
		defer resp.Body.Close()

		// Check status
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			file.Close()
			return nil
		}

		// 4xx errors should not be retried
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			file.Close()
			return fmt.Errorf("upload failed with status %d", resp.StatusCode)
		}

		// 5xx errors can be retried
		lastErr = fmt.Errorf("upload failed with status %d", resp.StatusCode)
		file.Close()
	}

	return fmt.Errorf("upload failed after %d retries: %w", maxRetries, lastErr)
}
```

**Step 2: Add required imports**

Update imports:

```go
import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"  // Add this

	"github.com/Khan/genqlient/graphql"  // Add this
	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"  // Add this
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
)
```

**Step 3: Build to verify**

Run: `make build`
Expected: Clean build

**Step 4: Commit**

```bash
git add cmd/attachment.go
git commit -m "feat: add file upload helpers with progress and retry

- Upload files through Linear's pre-signed URL flow
- Show progress bar for all uploads
- Retry up to 3 times with exponential backoff
- Convert Linear headers to HTTP headers
- Create attachment after successful upload"
```

---

### Task 8: Implement Attachment Upload Command

**Files:**
- Modify: `cmd/attachment.go`

**Step 1: Add upload subcommand with flag parsing**

Append to `cmd/attachment.go`:

```go
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

		// Parse file attachments from flags
		files, err := parseFileFlags(cmd)
		if err != nil {
			output.Error(jsonOutput, plaintextOutput, err.Error())
			os.Exit(1)
		}

		if len(files) == 0 {
			output.Error(jsonOutput, plaintextOutput, "At least one --file and --title pair is required")
			os.Exit(1)
		}

		// Validate all files first
		fmt.Println("Validating files...")
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

			fmt.Printf("✓ %s (%s) - OK\n", filepath.Base(files[i].path), formatSize(size))
		}

		// Stop if validation failed
		if len(validationErrors) > 0 {
			fmt.Println("\nError: Validation failed:")
			for _, ve := range validationErrors {
				fmt.Printf("  - %s: %s\n", ve.filename, ve.error)
			}
			os.Exit(1)
		}

		// Get auth and create client
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(jsonOutput, plaintextOutput, err.Error())
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		// Upload files
		fmt.Println()
		var succeeded, failed int
		var failures []string

		for _, file := range files {
			err := uploadFileToLinear(ctx, client, file, issueID)
			if err != nil {
				fmt.Printf("✗ Failed to attach %s: %v\n", filepath.Base(file.path), err)
				failed++
				failures = append(failures, fmt.Sprintf("%s: %v", filepath.Base(file.path), err))
			} else {
				fmt.Printf("✓ Attached %s to %s\n", filepath.Base(file.path), issueID)
				succeeded++
			}
			fmt.Println()
		}

		// Summary
		fmt.Printf("Summary: %d succeeded, %d failed\n", succeeded, failed)
		if len(failures) > 0 {
			fmt.Println("Failed uploads:")
			for _, f := range failures {
				fmt.Printf("  - %s\n", f)
			}
			os.Exit(1)
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
```

**Step 2: Build and test help**

Run: `go run main.go attachment upload --help`
Expected: Shows help with multiple --file support

**Step 3: Build**

Run: `make build`
Expected: Clean build

**Step 4: Commit**

```bash
git add cmd/attachment.go
git commit -m "feat: implement attachment upload command

- Upload multiple files with individual metadata
- Validate all files before uploading any
- Show progress for each upload
- Continue with remaining files if one fails
- Support retry logic with exponential backoff
- Summarize results at end"
```

---

### Task 9: Implement Attachment Update Command

**Files:**
- Modify: `cmd/attachment.go`

**Step 1: Add update subcommand**

Append to `cmd/attachment.go`:

```go
var attachmentUpdateCmd = &cobra.Command{
	Use:   "update <attachment-id>",
	Short: "Update an attachment's metadata",
	Long: `Update an attachment's title, subtitle, icon, or metadata.

For file attachments, use --file to re-upload.
For URL attachments, use --url to change the URL.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		attachmentID := args[0]

		// Get flags
		title, _ := cmd.Flags().GetString("title")
		subtitle, _ := cmd.Flags().GetString("subtitle")
		iconURL, _ := cmd.Flags().GetString("icon-url")
		metadataStr, _ := cmd.Flags().GetString("metadata")
		url, _ := cmd.Flags().GetString("url")
		filePath, _ := cmd.Flags().GetString("file")

		// Check if any update specified
		if !cmd.Flags().Changed("title") && !cmd.Flags().Changed("subtitle") &&
		   !cmd.Flags().Changed("icon-url") && !cmd.Flags().Changed("metadata") &&
		   !cmd.Flags().Changed("url") && !cmd.Flags().Changed("file") {
			output.Error(jsonOutput, plaintextOutput, "No fields to update (specify --title, --subtitle, --icon-url, --metadata, --url, or --file)")
			os.Exit(1)
		}

		// Get auth and create client
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(jsonOutput, plaintextOutput, err.Error())
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		// Handle file re-upload
		if filePath != "" {
			// Validate file
			size, err := validateFile(filePath)
			if err != nil {
				output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("File validation failed: %v", err))
				os.Exit(1)
			}

			contentType, err := detectContentType(filePath)
			if err != nil {
				output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("Failed to detect content type: %v", err))
				os.Exit(1)
			}

			// Upload new file
			uploadResp, err := api.FileUpload(ctx, client, contentType, filepath.Base(filePath), int(size))
			if err != nil {
				output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("Failed to get upload URL: %v", err))
				os.Exit(1)
			}

			if err := uploadFileWithProgress(filePath, uploadResp.FileUpload.UploadUrl, uploadResp.FileUpload.Headers, size); err != nil {
				output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("Upload failed: %v", err))
				os.Exit(1)
			}

			// Use new asset URL
			url = uploadResp.FileUpload.AssetUrl
		}

		// Build update input
		input := api.AttachmentUpdateInput{}
		if cmd.Flags().Changed("title") {
			input.Title = &title
		}
		if cmd.Flags().Changed("subtitle") {
			input.Subtitle = &subtitle
		}
		if cmd.Flags().Changed("icon-url") {
			input.IconUrl = &iconURL
		}
		if cmd.Flags().Changed("url") || url != "" {
			input.Url = &url
		}
		if cmd.Flags().Changed("metadata") {
			metadata, err := parseMetadata(metadataStr)
			if err != nil {
				output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("Invalid metadata: %v", err))
				os.Exit(1)
			}
			input.Metadata = metadata
		}

		// Call API
		resp, err := api.AttachmentUpdate(ctx, client, attachmentID, input)
		if err != nil {
			output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("Failed to update attachment: %v", err))
			os.Exit(1)
		}

		if !resp.AttachmentUpdate.Success {
			output.Error(jsonOutput, plaintextOutput, "Failed to update attachment")
			os.Exit(1)
		}

		// Render output
		if jsonOutput {
			output.JSON(resp.AttachmentUpdate.Attachment)
			return
		}

		fmt.Printf("✓ Updated attachment: %s\n", resp.AttachmentUpdate.Attachment.Title)
	},
}

func init() {
	attachmentCmd.AddCommand(attachmentUpdateCmd)
	attachmentUpdateCmd.Flags().String("title", "", "New title")
	attachmentUpdateCmd.Flags().String("subtitle", "", "New subtitle")
	attachmentUpdateCmd.Flags().String("icon-url", "", "New icon URL")
	attachmentUpdateCmd.Flags().String("metadata", "", "New metadata as key=value pairs")
	attachmentUpdateCmd.Flags().String("url", "", "New URL (for URL attachments)")
	attachmentUpdateCmd.Flags().String("file", "", "Re-upload file (for file attachments)")
}
```

**Step 2: Build**

Run: `make build`
Expected: Clean build

**Step 3: Commit**

```bash
git add cmd/attachment.go
git commit -m "feat: implement attachment update command

- Update title, subtitle, icon-url, metadata
- Re-upload files for file attachments
- Change URL for URL attachments
- Validate at least one field is being updated"
```

---

### Task 10: Implement Attachment Delete Command

**Files:**
- Modify: `cmd/attachment.go`

**Step 1: Add delete subcommand**

Append to `cmd/attachment.go`:

```go
var attachmentDeleteCmd = &cobra.Command{
	Use:   "delete <attachment-id>",
	Short: "Delete an attachment",
	Long:  `Delete an attachment from an issue. This action cannot be undone.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		attachmentID := args[0]

		// Get auth and create client
		authHeader, err := auth.GetAuthHeader()
		if err != nil {
			output.Error(jsonOutput, plaintextOutput, err.Error())
			os.Exit(1)
		}

		client := api.NewClient(authHeader)
		ctx := context.Background()

		// Call API
		resp, err := api.AttachmentDelete(ctx, client, attachmentID)
		if err != nil {
			output.Error(jsonOutput, plaintextOutput, fmt.Sprintf("Failed to delete attachment: %v", err))
			os.Exit(1)
		}

		if !resp.AttachmentDelete.Success {
			output.Error(jsonOutput, plaintextOutput, "Failed to delete attachment")
			os.Exit(1)
		}

		// Render output
		if jsonOutput {
			output.JSON(map[string]bool{"success": true})
			return
		}

		fmt.Printf("✓ Deleted attachment %s\n", attachmentID)
	},
}

func init() {
	attachmentCmd.AddCommand(attachmentDeleteCmd)
}
```

**Step 2: Build**

Run: `make build`
Expected: Clean build

**Step 3: Commit**

```bash
git add cmd/attachment.go
git commit -m "feat: implement attachment delete command

- Delete attachments by ID
- No confirmation prompt (fast and scriptable)
- Show success message or error"
```

---

### Task 11: Add Attachment Commands to Smoke Tests

**Files:**
- Modify: `smoke_test.sh`

**Step 1: Add list command tests**

Find the section with other list commands and add:

```bash
# Attachment list (read-only, uses test issue)
echo "Testing: lincli attachment list"
./lincli attachment list "$TEST_ISSUE_ID" > /dev/null || fail "attachment list failed"

echo "Testing: lincli attachment list --json"
./lincli attachment list "$TEST_ISSUE_ID" --json > /dev/null || fail "attachment list --json failed"

echo "Testing: lincli attachment list --plaintext"
./lincli attachment list "$TEST_ISSUE_ID" --plaintext > /dev/null || fail "attachment list --plaintext failed"
```

**Step 2: Define TEST_ISSUE_ID if not exists**

Near the top of `smoke_test.sh`, check if TEST_ISSUE_ID is defined. If not, add:

```bash
# Use a known issue ID for attachment tests (update this to a real issue in your workspace)
TEST_ISSUE_ID="${TEST_ISSUE_ID:-LIN-1}"
```

**Step 3: Run smoke tests**

Run: `./smoke_test.sh`
Expected: All tests pass (including new attachment list tests)

**Step 4: Commit**

```bash
git add smoke_test.sh
git commit -m "test: add attachment list to smoke tests

- Test list command with default output
- Test JSON and plaintext formats
- Use TEST_ISSUE_ID environment variable"
```

---

### Task 12: Update README Documentation

**Files:**
- Modify: `README.md`

**Step 1: Add attachment commands to Quick Start**

Find the "Quick Start" section and add after the Comments section:

```markdown
### 7. Attachment Management
```bash
# List attachments on an issue
lincli attachment list LIN-123

# Create URL attachment
lincli attachment create LIN-123 \
  --url "https://github.com/org/repo/pull/456" \
  --title "Fix PR"

# Upload files
lincli attachment upload LIN-123 \
  --file report.pdf --title "Q4 Report" \
  --file screenshot.png --title "Error Screenshot"

# Update attachment
lincli attachment update <attachment-id> --title "Updated Title"

# Delete attachment
lincli attachment delete <attachment-id>
```
```

**Step 2: Add to Command Reference**

Find the "Command Reference" section and add:

```markdown
### Attachment Commands
```bash
# List attachments
lincli attachment list <issue-id> [flags]
lincli attachment ls <issue-id> [flags]     # Alias
# Flags:
  -l, --limit int          Maximum results (default 50)
  -o, --sort string        Sort order: created (default), updated

# Create URL attachment
lincli attachment create <issue-id> [flags]
# Flags:
  --url string             URL to attach (required)
  --title string           Attachment title (required)
  --subtitle string        Attachment subtitle
  --icon-url string        Custom icon URL
  --metadata string        Metadata as key=value pairs (comma-separated)

# Upload files
lincli attachment upload <issue-id> [flags]
# Flags:
  --file string[]          Path to file (required, repeatable)
  --title string[]         Attachment title for each file (required)
  --subtitle string[]      Attachment subtitle
  --icon-url string[]      Custom icon URL
  --metadata string[]      Metadata as key=value pairs

# Update attachment
lincli attachment update <attachment-id> [flags]
# Flags:
  --title string           New title
  --subtitle string        New subtitle
  --icon-url string        New icon URL
  --metadata string        New metadata
  --url string             New URL (URL attachments only)
  --file string            Re-upload file (file attachments only)

# Delete attachment
lincli attachment delete <attachment-id>
```
```

**Step 3: Add to Features list**

Find the Features section at the top and add:

```markdown
- 📎 **Attachment Management**: Upload files, create URL attachments, full CRUD operations
  - File uploads to Linear's storage with progress tracking
  - URL attachments for external resources (GitHub PRs, docs, etc.)
  - 50MB file size limit with retry logic
  - List, update, and delete attachments
```

**Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add attachment management to README

- Add attachment commands to Quick Start
- Add full command reference for attachments
- Add to Features list
- Include examples for all operations"
```

---

### Task 13: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add to Adding New Commands section**

Find the section about adding commands and add a note:

```markdown
### Attachment Commands Example

See `cmd/attachment.go` for a complete example of:
- Multiple subcommands under one parent command
- File upload with progress tracking and retry logic
- Multi-step operations (GraphQL mutation → HTTP PUT → GraphQL mutation)
- Flag parsing for multiple values with relationships (--file, --title pairs)
- Validation before API calls
- Best-effort error handling (continue with remaining items on failure)
```

**Step 2: Add to Common Patterns section**

Add after existing patterns:

```markdown
### File Upload Pattern (cmd/attachment.go)

For operations requiring file uploads:
1. Validate files locally first (size, exists, readable)
2. Call `FileUpload` mutation to get pre-signed URL
3. HTTP PUT file to pre-signed URL with progress tracking
4. Call operation-specific mutation with asset URL
5. Handle retries for network errors (3 attempts, exponential backoff)
6. Continue with remaining files if one fails
```

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: document attachment implementation patterns

- Add attachment commands as example of multi-subcommand structure
- Document file upload pattern with retry logic
- Add notes on validation and error handling"
```

---

### Task 14: Final Verification

**Files:**
- None (testing only)

**Step 1: Build project**

Run: `make build`
Expected: Clean build, no errors

**Step 2: Run smoke tests**

Run: `./smoke_test.sh`
Expected: All tests pass (39 tests total including 3 new attachment tests)

**Step 3: Verify all commands exist**

Run: `./lincli attachment --help`
Expected: Shows 5 subcommands (list, create, upload, update, delete)

**Step 4: Check command help**

Run each:
```bash
./lincli attachment list --help
./lincli attachment create --help
./lincli attachment upload --help
./lincli attachment update --help
./lincli attachment delete --help
```
Expected: Each shows proper help with flags

**Step 5: Verification complete**

If all checks pass, implementation is complete and ready for manual testing.

---

## Manual Testing Checklist

After implementation, perform these manual tests (using real Linear API):

1. **Create test issue**: `lincli issue create --title "Attachment Testing" --team <your-team> --assign-me`

2. **Upload single file**:
   - Small file (<1MB)
   - Large file (close to 50MB)
   - Various types (pdf, png, jpg, txt)

3. **Upload multiple files**: Test with 2-3 files, each with different metadata

4. **Create URL attachment**: Link to GitHub PR or external doc

5. **List attachments**: Verify both file and URL attachments appear

6. **Update attachment**: Change title, subtitle, metadata

7. **Re-upload file**: Update file attachment with new file

8. **Delete attachment**: Remove test attachments

9. **Error cases**:
   - Upload >50MB file (should fail validation)
   - Missing --title flag (should error)
   - Invalid metadata format (should error)
   - Non-existent issue ID (should error)

10. **JSON output**: Test all commands with --json flag

---

## Success Criteria

- ✓ All 5 attachment commands implemented
- ✓ File uploads work with progress and retry
- ✓ 50MB limit enforced
- ✓ Multiple files with individual metadata supported
- ✓ All commands support --json and --plaintext
- ✓ Smoke tests pass
- ✓ Documentation updated
- ✓ Manual testing checklist completed
