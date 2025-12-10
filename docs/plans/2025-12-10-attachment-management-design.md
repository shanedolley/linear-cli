# Attachment Management Design

**Date:** 2025-12-10
**Status:** Approved
**Author:** Design session with user

## Overview

Add comprehensive attachment management to lincli, supporting both file uploads and URL attachments. Users can upload files to Linear's storage, create URL attachments, and manage all attachments through CRUD operations.

## Command Structure

### 1. Upload Files
```bash
lincli attachment upload <issue-id> \
  --file report.pdf --title "Q4 Report" --subtitle "Draft" \
  --file screenshot.png --title "Bug Screenshot" \
  --icon-url "https://..." \
  --metadata key1=value1,key2=value2
```

**Features:**
- Multiple files per command, each with own metadata
- Required: `--file` and `--title` for each attachment
- Optional: `--subtitle`, `--icon-url`, `--metadata`
- File validation: 50MB max size, exists and readable
- Always shows upload progress (even for small files)
- Pre-validates all files before uploading any
- Continues uploading remaining files if one fails
- Retries up to 3 times on network errors

### 2. Create URL Attachments
```bash
lincli attachment create <issue-id> \
  --url "https://github.com/org/repo/pull/123" \
  --title "PR #123" \
  --subtitle "Merged" \
  --icon-url "https://..." \
  --metadata key=value
```

**Features:**
- One URL attachment per command
- Same metadata fields as file uploads for consistency
- Required: `--url` and `--title`

### 3. List Attachments
```bash
lincli attachment list <issue-id> [flags]
```

**Flags:**
- `--limit` - Maximum results
- `--sort` - Sort by created or updated
- `--json`, `--plaintext` - Output format

**Features:**
- Follows standard lincli list patterns
- Shows both file and URL attachments
- Supports pagination

### 4. Update Attachments
```bash
lincli attachment update <attachment-id> \
  --title "New Title" \
  --subtitle "Updated" \
  --file new-report.pdf  # Re-upload for file attachments
  --url "https://..."     # Update URL for URL attachments
```

**Features:**
- Can update all fields
- For file attachments: `--file` triggers re-upload
- For URL attachments: `--url` changes the URL
- Metadata can be updated for both types

### 5. Delete Attachments
```bash
lincli attachment delete <attachment-id>
```

**Features:**
- No confirmation prompt (fast and scriptable)
- Returns clear error if attachment doesn't exist

## Technical Implementation

### GraphQL Operations

New file: `pkg/api/operations/attachments.graphql`

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

# List attachments
query ListAttachments($issueId: ID!, $first: Int, $after: String) {
  issue(id: $issueId) {
    attachments(first: $first, after: $after) {
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

# Update attachment
mutation AttachmentUpdate($id: ID!, $input: AttachmentUpdateInput!) {
  attachmentUpdate(id: $id, input: $input) {
    success
    attachment {
      id
      title
      subtitle
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

### File Upload Flow

Multi-step process for each file:

1. **Validate files locally** (before any API calls)
   - Check file exists and is readable
   - Check size ≤ 50MB
   - Detect content type using `mime.TypeByExtension()` or `http.DetectContentType()`

2. **For each file:**
   - Call `FileUpload` mutation → get `uploadUrl`, `assetUrl`, `headers`
   - HTTP PUT file to `uploadUrl` with headers and progress tracking
   - Call `AttachmentCreate` mutation with `assetUrl` to attach to issue

3. **Error handling:**
   - Validation failures → stop before uploading anything
   - Upload failures → retry up to 3 times, then continue with remaining files

### Code Structure

New file: `cmd/attachment.go`

Following existing lincli patterns:
- `attachmentCmd` - root command
- `attachmentUploadCmd` - file upload handler
- `attachmentCreateCmd` - URL attachment handler
- `attachmentListCmd` - list handler
- `attachmentUpdateCmd` - update handler
- `attachmentDeleteCmd` - delete handler

### Key Implementation Details

- **File validation**: Use `os.Stat()` for file checks
- **Content type detection**: Use `mime.TypeByExtension()` fallback to `http.DetectContentType()`
- **HTTP PUT**: Use `net/http` package (not GraphQL) for file upload
- **Progress tracking**: Use progress bar library (e.g., `github.com/schollz/progressbar/v3`)
- **Flag parsing**: Track state to group `--file`, `--title` pairs for multiple files
- **Type safety**: Use genqlient-generated types throughout

### Retry Logic

For network errors during file upload:
- Retry up to 3 times for transient network errors
- Exponential backoff: 1s, 2s, 4s between retries
- **Do NOT retry** on client errors (4xx status codes)
- Restart upload from beginning on each retry (pre-signed URLs are single-use)
- Show retry attempts in progress output

## Error Handling

### Validation Errors (fail before uploads)

```bash
# File doesn't exist
Error: Validation failed:
  - missing.pdf: file not found

# File too large
Error: Validation failed:
  - huge.bin: file size 75.3 MB exceeds limit of 50 MB

# File not readable
Error: Validation failed:
  - locked.pdf: permission denied

# Missing required flags
Error: --title is required for each file
```

### Upload Errors (continue with remaining)

```bash
Validating files...
✓ report.pdf (2.3 MB) - OK
✓ screenshot.png (145 KB) - OK
✓ doc.pdf (8.1 MB) - OK

Uploading report.pdf (2.3 MB)...
[████████████████████████] 100% (2.3 MB / 2.3 MB)
✓ Attached report.pdf to LIN-123

Uploading screenshot.png (145 KB)...
[████████████░░░░░░░░░░░░] 50% (72 KB / 145 KB)
✗ Network error, retrying... (attempt 1/3)
[███░░░░░░░░░░░░░░░░░░░░░] 15% (21 KB / 145 KB)
✗ Network error, retrying... (attempt 2/3)
[████████████████████████] 100% (145 KB / 145 KB)
✓ Attached screenshot.png to LIN-123 (succeeded after 2 retries)

Uploading doc.pdf (8.1 MB)...
[████████████████████████] 100% (8.1 MB / 8.1 MB)
✓ Attached doc.pdf to LIN-123

Summary: 3 succeeded, 0 failed
```

### Edge Cases

- **Empty files**: Allow (0 bytes), Linear may reject
- **Special characters in filenames**: URL-encode as needed
- **Very long filenames**: Truncate for display, preserve for upload
- **Duplicate files**: Each upload creates separate attachment
- **Issue doesn't exist**: API returns error before any upload
- **Network interruption**: Retry with exponential backoff
- **Invalid attachment ID**: Show Linear's error message

### Metadata Parsing

```bash
# Comma-separated key=value pairs
--metadata "exceptionId=exc-123,severity=high,resolved=false"

# Error on invalid format
--metadata "invalid-format"
Error: metadata must be key=value pairs separated by commas
```

### Update Edge Cases

```bash
# Can't re-upload file for URL attachment
lincli attachment update <url-attachment-id> --file report.pdf
Error: Cannot upload file for URL attachment (use --url to change URL)

# Can't change URL for file attachment
lincli attachment update <file-attachment-id> --url "https://..."
Error: Cannot change URL for file attachment (use --file to re-upload)

# Update without changes
lincli attachment update <id>
Error: No fields to update (specify --title, --subtitle, etc.)
```

## Testing Strategy

### Unit Tests

New file: `tests/unit/attachment_test.go`

Test coverage:
- File validation logic (exists, readable, size)
- Content type detection
- Metadata parsing (key=value pairs)
- Flag parsing for multiple files
- Progress calculation
- Retry backoff timing

### Integration Tests (Read-only)

Add to `smoke_test.sh`:
```bash
# List attachments (safe, read-only)
lincli attachment list <test-issue-id> --json
lincli attachment list <test-issue-id> --plaintext
lincli attachment list <test-issue-id> --limit 10 --sort created
```

### Manual Testing Checklist

Must verify before merging:

**Single file upload:**
- [ ] Small file (<1MB)
- [ ] Large file (30-40MB, test progress bar)
- [ ] Various file types (pdf, png, jpg, txt, zip)

**Multiple file upload:**
- [ ] 2-3 files with different metadata
- [ ] One file fails validation (others shouldn't upload)
- [ ] One file fails upload (others should continue)

**URL attachments:**
- [ ] GitHub PR URL
- [ ] External documentation URL
- [ ] With custom icon URL

**List attachments:**
- [ ] Issue with no attachments
- [ ] Issue with mix of file and URL attachments
- [ ] JSON output for scripting
- [ ] Sorting by created/updated

**Update attachments:**
- [ ] Update title/subtitle
- [ ] Re-upload file attachment
- [ ] Change URL for URL attachment
- [ ] Update metadata

**Delete attachments:**
- [ ] Delete file attachment
- [ ] Delete URL attachment
- [ ] Delete non-existent attachment (error handling)

**Edge cases:**
- [ ] Empty file (0 bytes)
- [ ] Filename with spaces/special characters
- [ ] Network interruption simulation (retry logic)
- [ ] 50MB+ file (should fail validation)
- [ ] Missing required flags

### Test Issue Setup

Before manual testing:
```bash
lincli issue create --title "Attachment Testing" --team <your-team> --assign-me
# Use this issue ID for all attachment tests
```

## Dependencies

New Go dependencies:
- Progress bar library: `github.com/schollz/progressbar/v3` (or similar)
- Standard library packages already available:
  - `os` - file operations
  - `mime` - content type detection
  - `net/http` - HTTP PUT requests

## Documentation Updates

Files to update:
- `README.md` - Add attachment command examples
- `CLAUDE.md` - Add attachment implementation notes
- `cmd/attachment.go` - Inline command documentation

## Success Criteria

- [ ] All five attachment commands implemented (`upload`, `create`, `list`, `update`, `delete`)
- [ ] File uploads work with progress tracking and retry logic
- [ ] 50MB file size limit enforced
- [ ] Multiple file uploads work with individual metadata
- [ ] All commands support `--json` and `--plaintext` output
- [ ] Error messages are clear and actionable
- [ ] Manual testing checklist completed
- [ ] Smoke tests pass for read-only operations
- [ ] Documentation updated
