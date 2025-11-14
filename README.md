# 🚀 lincli - Linear CLI Tool


A comprehensive command-line interface for Linear's API, built with agents in mind (but nice for humans too).

## ✨ Features

- 🔐 **Authentication**: Personal API Key support
- 📋 **Issue Management**: Create, list, view, update, assign, and manage issues with full details
  - Sub-issue hierarchy with parent/child relationships
  - Git branch integration showing linked branches
  - Cycle (sprint) and project associations
  - Attachments and recent comments preview
  - Due dates, snoozed status, and completion tracking
  - Full-text search via `lincli issue search`
- 👥 **Team Management**: View teams, get team details, and list team members
- 🚀 **Project Tracking**: Comprehensive project information
  - Progress visualization with issue statistics
  - Team and member associations
  - Initiative hierarchy
  - Recent issues preview
  - Timeline tracking (created, updated, completed dates)
- 👤 **User Management**: List all users, view user details, and current user info
- 💬 **Comments**: List and create comments on issues with time-aware formatting
- 📎 **Attachments**: View file uploads and attachments on issues
- 🔗 **Webhooks**: Configure and manage webhooks
- 🎨 **Multiple Output Formats**: Table, plaintext, and JSON output
- ⚡ **Performance**: Fast and lightweight CLI tool
- 🔄 **Flexible Sorting**: Sort lists by Linear's default order, creation date, or update date
- 📅 **Time-based Filtering**: Filter lists by creation date with intuitive time expressions
- 📚 **Built-in Documentation**: Access full documentation with `lincli docs`
- 🧪 **Smoke Testing**: Automated smoke tests for all read-only commands

## 🛠️ Installation

### Homebrew (macOS/Linux)
```bash
brew tap shanedolley/lincli
brew install lincli
lincli docs      # Render the README.md
```

### From Source
```bash
git clone https://github.com/shanedolley/lincli.git
cd lincli
make deps        # Install dependencies
make build       # Build the binary
make install     # Install to /usr/local/bin (requires sudo)
lincli docs      # Render the README.md
```

### For Development
```bash
git clone https://github.com/shanedolley/lincli.git
cd lincli
make deps        # Install dependencies
go run main.go   # Run directly without building
make dev         # Or build and run in development mode
make test        # Run all tests
make lint        # Run linter
make fmt         # Format code
lincli docs      # Render the README.md
```

## Important: Default Filters

**By default, `issue list`, `issue search`, and `project list` commands only show items created in the last 6 months!**
 
This improves performance and prevents overwhelming data loads. To see older items:
 - Use `--newer-than 1_year_ago` for items from the last year
 - Use `--newer-than all_time` to see ALL items ever created
 - See the [Time-based Filtering](#-time-based-filtering) section for details

**By default, `issue list` and `issue search` also filter out canceled and completed items. To see all items, use the `--include-completed` flag.**
- Need archived matches? Add `--include-archived` when using `issue search`.


## 🚀 Quick Start

> **IMPORTANT**  Agents like Claude Code, Cursor, and Gemini should use the `--json` flag on all read operations.

### 1. Authentication
```bash
# Interactive authentication
lincli auth

# Check authentication status
lincli auth status

# Show current user
lincli whoami

# View full documentation
lincli docs | less
```

### 2. Issue Management
```bash
# List all issues
lincli issue list

# List issues assigned to you
lincli issue list --assignee me

# List issues in a specific state
lincli issue list --state "In Progress"

# List issues sorted by update date
lincli issue list --sort updated

# Search issues using Linear's full-text index (shares the same filters as list)
lincli issue search "login bug" --team ENG
lincli issue search "customer:" --include-completed --include-archived

# List recent issues (last 2 weeks instead of default 6 months)
lincli issue list --newer-than 2_weeks_ago

# List ALL issues ever created (override 6-month default)
lincli issue list --newer-than all_time

# List today's issues
lincli issue list --newer-than 1_day_ago

# Get issue details (now includes git branch, cycle, project, attachments, and comments)
lincli issue get LIN-123

# Create a new issue
lincli issue create --title "Bug fix" --team ENG

# Assign issue to yourself
lincli issue assign LIN-123

# Update issue fields
lincli issue update LIN-123 --title "New title"
lincli issue update LIN-123 --description "Updated description"
lincli issue update LIN-123 --assignee john.doe@company.com
lincli issue update LIN-123 --assignee me  # Assign to yourself
lincli issue update LIN-123 --assignee unassigned  # Remove assignee
lincli issue update LIN-123 --state "In Progress"
lincli issue update LIN-123 --priority 1  # 0=None, 1=Urgent, 2=High, 3=Normal, 4=Low
lincli issue update LIN-123 --due-date "2024-12-31"
lincli issue update LIN-123 --due-date ""  # Remove due date

# Update multiple fields at once
lincli issue update LIN-123 --title "Critical Bug" --assignee me --priority 1
```

### 3. Project Management
```bash
# List all projects (shows IDs)
lincli project list

# Filter projects by team
lincli project list --team ENG

# List projects created in the last month (instead of default 6 months)
lincli project list --newer-than 1_month_ago

# List ALL projects regardless of age
lincli project list --newer-than all_time

# Get project details (use ID from list command)
lincli project get 65a77a62-ec5e-491e-b1d9-84aebee01b33
```

### 4. Team Management
```bash
# List all teams
lincli team list

# Get team details
lincli team get ENG

# List team members
lincli team members ENG
```

### 5. User Management
```bash
# List all users
lincli user list

# Show only active users
lincli user list --active

# Get user details by email
lincli user get john@example.com

# Show your own profile
lincli user me
```

### 6. Comments
```bash
# List comments on an issue
lincli comment list LIN-123

# Add a comment to an issue
lincli comment create LIN-123 --body "Fixed the authentication bug"
```

## 📖 Command Reference

### Global Flags
- `--plaintext, -p`: Plain text output (non-interactive)
- `--json, -j`: JSON output for scripting
- `--help, -h`: Show help
- `--version, -v`: Show version

### Authentication Commands
```bash
lincli auth               # Interactive authentication
lincli auth login         # Same as above
lincli auth status        # Check authentication status
lincli auth logout        # Clear stored credentials
lincli whoami            # Show current user
```

### Issue Commands
```bash
# List issues with filters
lincli issue list [flags]
lincli issue ls [flags]     # Short alias

# Flags:
  -a, --assignee string     Filter by assignee (email or 'me')
  -c, --include-completed   Include completed and canceled issues
  -s, --state string       Filter by state name
  -t, --team string        Filter by team key
  -r, --priority int       Filter by priority (0-4, default: -1)
  -l, --limit int          Maximum results (default 50)
  -o, --sort string        Sort order: linear (default), created, updated
  -n, --newer-than string  Show items created after this time (default: 6_months_ago, use 'all_time' for no filter)

# Get issue details (shows parent and sub-issues)
lincli issue get <issue-id>
lincli issue show <issue-id>  # Alias

# Create issue
lincli issue create [flags]
lincli issue new [flags]      # Alias
# Flags:
  --title string           Issue title (required)
  -d, --description string Issue description
  -t, --team string        Team key (required)
  --priority int       Priority 0-4 (default 3)
  -m, --assign-me          Assign to yourself

# Assign issue to yourself
lincli issue assign <issue-id>

# Update issue
lincli issue update <issue-id> [flags]
lincli issue edit <issue-id> [flags]    # Alias
# Flags:
  --title string           New title
  -d, --description string New description
  -a, --assignee string    Assignee (email, name, 'me', or 'unassigned')
  -s, --state string       State name (e.g., 'Todo', 'In Progress', 'Done')
  --priority int           Priority (0=None, 1=Urgent, 2=High, 3=Normal, 4=Low)
  --due-date string        Due date (YYYY-MM-DD format, or empty to remove)

# Archive issue (coming soon)
lincli issue archive <issue-id>
```

### Team Commands
```bash
# List all teams with issue counts
lincli team list
lincli team ls              # Alias
# Flags:
  -l, --limit int          Maximum results (default 50)
  -o, --sort string        Sort order: linear (default), created, updated

# Get team details
lincli team get <team-key>
lincli team show <team-key> # Alias

# Examples:
lincli team get ENG         # Shows Engineering team details
lincli team get DESIGN      # Shows Design team details

# List team members with roles and status
lincli team members <team-key>

# Examples:
lincli team members ENG     # Lists all Engineering team members
```

### Project Commands
```bash
# List projects
lincli project list [flags]
lincli project ls [flags]     # Alias
# Flags:
  -t, --team string        Filter by team key
  -s, --state string       Filter by state (planned, started, paused, completed, canceled)
  -l, --limit int          Maximum results (default 50)
  -o, --sort string        Sort order: linear (default), created, updated
  -n, --newer-than string  Show items created after this time (default: 6_months_ago)
  -c, --include-completed  Include completed and canceled projects

# Get project details
lincli project get <project-id>
lincli project show <project-id>  # Alias

# Create project (coming soon)
lincli project create [flags]
```

### User Commands
```bash
# List all users in workspace
lincli user list [flags]
lincli user ls [flags]      # Alias
# Flags:
  -a, --active             Show only active users
  -l, --limit int          Maximum results (default 50)
  -o, --sort string        Sort order: linear (default), created, updated

# Examples:
lincli user list            # List all users
lincli user list --active   # List only active users

# Get user details by email
lincli user get <email>
lincli user show <email>    # Alias

# Examples:
lincli user get john@example.com
lincli user get jane.doe@company.com

# Show current authenticated user
lincli user me              # Shows your profile with admin status
```

### Comment Commands
```bash
# List all comments for an issue
lincli comment list <issue-id> [flags]
lincli comment ls <issue-id> [flags]    # Alias
# Flags:
  -l, --limit int          Maximum results (default 50)
  -o, --sort string        Sort order: linear (default), created, updated

# Examples:
lincli comment list LIN-123      # Shows all comments with timestamps
lincli comment list LIN-456 -l 10 # Show latest 10 comments

# Add comment to issue
lincli comment create <issue-id> --body "Comment text"
lincli comment add <issue-id> -b "Comment text"    # Alias
lincli comment new <issue-id> -b "Comment text"    # Alias

# Examples:
lincli comment create LIN-123 --body "I've started working on this"
lincli comment add LIN-123 -b "Fixed in commit abc123"
lincli comment create LIN-456 --body "@john please review this PR"
```

## 🎨 Output Formats

### Table Format (Default)
```bash
lincli issue list
```
```
ID       Title                State        Assignee    Team  Priority
LIN-123  Fix authentication   In Progress  john@co.com ENG   High
LIN-124  Update documentation Done         jane@co.com DOC   Normal
```

### Plaintext Format
```bash
lincli issue list --plaintext
```
```
# Issues
## BUG: Fix login button alignment
- **ID**: FAK-123
- **State**: In Progress
- **Assignee**: Jane Doe
- **Team**: WEB
- **Created**: 2025-07-12
- **URL**: https://linear.app/example/issue/FAK-123/bug-fix-login-button-alignment
- **Description**: The login button on the main page is misaligned on mobile devices.

Steps to reproduce:
1. Open the website on a mobile browser.
2. Navigate to the login page.
3. Observe the button alignment.

## FEAT: Add dark mode support
- **ID**: FAK-124
- **State**: Todo
- **Assignee**: John Smith
- **Team**: APP
- **Created**: 2025-07-11
- **URL**: https://linear.app/example/issue/FAK-124/feat-add-dark-mode-support
- **Description**: Implement a dark mode theme for the entire application to improve user experience in low-light environments.
```

### JSON Format
```bash
lincli issue list --json
```
```json
[
  {
    "id": "LIN-123",
    "title": "Fix authentication",
    "state": "In Progress",
    "assignee": "john@co.com",
    "team": "ENG",
    "priority": "High"
  }
]
```

## ⚙️ Configuration

Configuration is stored in `~/.lincli.yaml`:

```yaml
# Default output format
output: table

# Default pagination limit
limit: 50

# API settings
api:
  timeout: 30s
  retries: 3
```

Authentication credentials are stored securely in `~/.lincli-auth.json`.

## 🔒 Authentication

### Personal API Key (Recommended)
1. Go to [Linear Settings > API](https://linear.app/settings/api)
2. Create a new Personal API Key
3. Run `lincli auth` and paste your key

## 📅 Time-based Filtering

**⚠️ Default Behavior**: To improve performance and prevent overwhelming data loads, list commands **only show items created in the last 6 months by default**. This is especially important for large workspaces.

### Using the --newer-than Flag

The `--newer-than` (or `-n`) flag is available on `issue list` and `project list` commands:

```bash
# Default behavior (last 6 months)
lincli issue list

# Show items from a specific time period
lincli issue list --newer-than 2_weeks_ago
lincli project list --newer-than 1_month_ago

# Show ALL items regardless of age
lincli issue list --newer-than all_time
```

### Supported Time Formats

1. **Relative time expressions**: `N_units_ago`
   - Units: `minutes`, `hours`, `days`, `weeks`, `months`, `years`
   - Examples: `30_minutes_ago`, `2_hours_ago`, `3_days_ago`, `1_week_ago`, `6_months_ago`

2. **Special values**:
   - `all_time` - Shows all items without any date filter
   - ISO dates - `2025-07-01` or `2025-07-01T15:30:00Z`

3. **Default value**: `6_months_ago` (when flag is not specified)

### Quick Reference

| Time Expression | Description | Example Command |
|----------------|-------------|-----------------|
| *(no flag)* | Last 6 months (default) | `lincli issue list` |
| `1_day_ago` | Last 24 hours | `lincli issue list --newer-than 1_day_ago` |
| `1_week_ago` | Last 7 days | `lincli issue list --newer-than 1_week_ago` |
| `2_weeks_ago` | Last 14 days | `lincli issue list --newer-than 2_weeks_ago` |
| `1_month_ago` | Last month | `lincli issue list --newer-than 1_month_ago` |
| `3_months_ago` | Last quarter | `lincli issue list --newer-than 3_months_ago` |
| `6_months_ago` | Last 6 months | `lincli issue list --newer-than 6_months_ago` |
| `1_year_ago` | Last year | `lincli issue list --newer-than 1_year_ago` |
| `all_time` | No date filter | `lincli issue list --newer-than all_time` |
| `2025-07-01` | Since specific date | `lincli issue list --newer-than 2025-07-01` |

### Common Use Cases

```bash
# Recent activity - issues from last week
lincli issue list --newer-than 1_week_ago

# Sprint planning - issues from current month
lincli issue list --newer-than 1_month_ago --state "Todo"

# Quarterly review - all projects from last 3 months
lincli project list --newer-than 3_months_ago

# Historical analysis - ALL issues ever created
lincli issue list --newer-than all_time --sort created

# Today's issues
lincli issue list --newer-than 1_day_ago

# Combine with other filters
lincli issue list --newer-than 2_weeks_ago --assignee me --sort updated
```

## 🔄 Sorting Options

All list commands support sorting with the `--sort` or `-o` flag:

- **linear** (default): Linear's built-in sorting order (respects manual ordering in the UI)
- **created**: Sort by creation date (newest first)
- **updated**: Sort by last update date (most recently updated first)

### Examples
```bash
# Get recently updated issues
lincli issue list --sort updated

# Get oldest projects first
lincli project list --sort created

# Get recently joined users
lincli user list --sort created --active

# Get latest comments on an issue
lincli comment list LIN-123 --sort created

# Combine sorting with filters
lincli issue list --assignee me --state "In Progress" --sort updated

# Combine time filtering with sorting
lincli issue list --newer-than 1_week_ago --sort updated

# Get all projects sorted by creation date
lincli project list --newer-than all_time --sort created
```

### Performance Tips

- The 6-month default filter significantly improves performance for large workspaces
- Use specific time ranges when possible instead of `all_time`
- Combine time filtering with other filters (assignee, state, team) for faster results

## 🧪 Testing

lincli includes comprehensive unit and integration tests to ensure reliability.

### Running Tests
```bash
# Run all tests  (currently just a smoke test)
make test
```

### Integration Testing
Integration tests require a Linear API key. Create a `.env.test` file:
```bash
cp .env.test.example .env.test
# Edit .env.test and add your LINEAR_TEST_API_KEY
```

Or set it as an environment variable:
```bash
export LINEAR_TEST_API_KEY="your-test-api-key"
make test-integration
```

⚠️ **Note**: Integration tests are read-only and safe to run with production API keys.

### Test Structure
- `tests/unit/` - Unit tests with mocked API responses
- `tests/integration/` - End-to-end tests with real Linear API
- `tests/testutils/` - Shared test utilities and helpers

See [tests/README.md](tests/README.md) for detailed testing documentation.

## 🤖 Scripting & Automation

Use `--plaintext` or `--json` flags for scripting:

```bash
#!/bin/bash

# Get all urgent issues in JSON format
urgent_issues=$(lincli issue list --priority 1 --json)

# Parse with jq
echo "$urgent_issues" | jq '.[] | select(.assignee == "me") | .id'

# Plaintext output for simple parsing
lincli issue list --assignee me --plaintext | cut -f1 | tail -n +2

# Get issue count for different time periods
echo "Last week: $(lincli issue list --newer-than 1_week_ago --json | jq '. | length')"
echo "Last month: $(lincli issue list --newer-than 1_month_ago --json | jq '. | length')"
echo "All time: $(lincli issue list --newer-than all_time --json | jq '. | length')"

# Create and assign issue in one command
lincli issue create --title "Fix bug" --team ENG --assign-me --json

# Get all projects for a team
lincli project list --team ENG --json | jq '.[] | {name, progress}'

# List all admin users
lincli user list --json | jq '.[] | select(.admin == true) | {name, email}'

# Get team member count
lincli team members ENG --json | jq '. | length'

# Export issue comments
lincli comment list LIN-123 --json > issue-comments.json
```

## 📡 Real-World Examples

### Team Workflows
```bash
# Find which team a user belongs to
for team in $(lincli team list --json | jq -r '.[].key'); do
  echo "Checking team: $team"
  lincli team members $team --json | jq '.[] | select(.email == "john@example.com")'
done

# List all private teams
lincli team list --json | jq '.[] | select(.private == true) | {key, name}'

# Get teams with more than 50 issues
lincli team list --json | jq '.[] | select(.issueCount > 50) | {key, name, issueCount}'
```

### User Management
```bash
# Find inactive users
lincli user list --json | jq '.[] | select(.active == false) | {name, email}'

# Check if you're an admin
lincli user me --json | jq '.admin'

# List users who are admins but not the current user
lincli user list --json | jq '.[] | select(.admin == true and .isMe == false) | .email'
```

### Issue Comments
```bash
# Add a comment mentioning the issue is blocked
lincli comment create LIN-123 --body "Blocked by LIN-456. Waiting for API changes."

# Get all comments by a specific user
lincli comment list LIN-123 --json | jq '.[] | select(.user.email == "john@example.com") | .body'

# Count comments per issue
for issue in LIN-123 LIN-124 LIN-125; do
  count=$(lincli comment list $issue --json | jq '. | length')
  echo "$issue: $count comments"
done
```

### Project Tracking
```bash
# List projects nearing completion (>80% progress)
lincli project list --json | jq '.[] | select(.progress > 0.8) | {name, progress}'

# Get all paused projects
lincli project list --state paused

# Show project timeline
lincli project get PROJECT-ID --json | jq '{name, startDate, targetDate, progress}'
```

### Daily Standup Helper
```bash
#!/bin/bash
# Show my recent activity
echo "=== My Issues ==="
lincli issue list --assignee me --limit 10

echo -e "\n=== Recent Comments ==="
for issue in $(lincli issue list --assignee me --json | jq -r '.[].identifier'); do
  echo "Comments on $issue:"
  lincli comment list $issue --limit 3
done
```

## 🐛 Troubleshooting

### Authentication Issues
```bash
# Check authentication status
lincli auth status

# Re-authenticate
lincli auth logout
lincli auth
```

### API Rate Limits
Linear has the following rate limits:
- Personal API Keys: 5,000 requests/hour

### Common Errors
- `Not authenticated`: Run `lincli auth` first
- `Team not found`: Use team key (e.g., "ENG") not display name
- `Invalid priority`: Use numbers 0-4 (0=None, 1=Urgent, 2=High, 3=Normal, 4=Low)

### Time Filtering Issues
- **Missing old issues?** Remember that list commands default to showing only the last 6 months
  - Solution: Use `--newer-than all_time` to see all issues
- **Invalid time expression?** Check the format: `N_units_ago` (e.g., `3_weeks_ago`)
  - Valid units: `minutes`, `hours`, `days`, `weeks`, `months`, `years`
- **Performance issues?** Avoid using `all_time` on large workspaces
  - Solution: Use specific time ranges like `--newer-than 1_year_ago`

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

See CONTRIBUTING.md for a detailed release checklist and the Homebrew tap auto-bump workflow.

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🔗 Links

- [Linear API Documentation](https://developers.linear.app/)
- [GitHub Repository](https://github.com/shanedolley/lincli)
- [Issue Tracker](https://github.com/shanedolley/lincli/issues)

---

**Built with ❤️ using Go, Cobra, and the Linear API**
