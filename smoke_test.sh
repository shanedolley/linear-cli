#!/bin/bash

# Smoke test for lincli - tests all READ commands
# This script runs through all the read-only commands to ensure basic functionality

set -e  # Exit on error

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local command="$2"
    local expected_pattern="$3"  # Optional pattern to check in output
    
    TESTS_RUN=$((TESTS_RUN + 1))
    echo -n "Testing: $test_name... "
    
    # Run the command and capture output and exit code
    set +e  # Temporarily disable exit on error
    output=$(eval "$command" 2>&1)
    exit_code=$?
    set -e
    
    if [ $exit_code -eq 0 ]; then
        # If an expected pattern is provided, check for it
        if [ -n "$expected_pattern" ]; then
            if echo "$output" | grep -q "$expected_pattern"; then
                echo -e "${GREEN}PASS${NC}"
                TESTS_PASSED=$((TESTS_PASSED + 1))
            else
                echo -e "${RED}FAIL${NC} - Expected pattern not found: $expected_pattern"
                echo "Output: $output" | head -5
                TESTS_FAILED=$((TESTS_FAILED + 1))
            fi
        else
            echo -e "${GREEN}PASS${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        fi
    else
        echo -e "${RED}FAIL${NC} - Exit code: $exit_code"
        echo "Error: $output" | head -5
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Function to extract first ID from list output
get_first_id() {
    local output="$1"
    # Extract first UUID-like pattern (for projects) or identifier like END-1234 (for issues)
    echo "$output" | grep -E -o '([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}|[A-Z]+-[0-9]+)' | head -1
}

echo "🚀 Starting lincli smoke tests..."
echo "================================"

# Check if authenticated
echo -e "\n${YELLOW}Checking authentication...${NC}"
run_test "auth status" "go run main.go auth status" "Authenticated"

# If not authenticated, skip tests
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "\n${RED}Not authenticated. Please run 'lincli auth' first.${NC}"
    exit 1
fi

# Test whoami
echo -e "\n${YELLOW}Testing user commands...${NC}"
run_test "whoami" "go run main.go whoami" "@"

# Test user list
run_test "user list" "go run main.go user list"
run_test "user list (plaintext)" "go run main.go user list -p"
run_test "user list (json)" "go run main.go user list -j" "\"email\""

# Note: user get requires user ID, not email - skipping for now
# Could be implemented by parsing JSON output from user list

# Test team commands
echo -e "\n${YELLOW}Testing team commands...${NC}"
run_test "team list" "go run main.go team list"
run_test "team list (plaintext)" "go run main.go team list -p"
run_test "team list (json)" "go run main.go team list -j" "\"key\""

# Get first team key for additional tests - look for pattern at start of line
team_key=$(go run main.go team list 2>/dev/null | awk 'NR>1 {print $1}' | head -1)
if [ -n "$team_key" ]; then
    run_test "team get" "go run main.go team get $team_key" "$team_key"
    run_test "team members" "go run main.go team members $team_key"
fi

# Test project commands
echo -e "\n${YELLOW}Testing project commands...${NC}"
run_test "project list" "go run main.go project list"
run_test "project list (plaintext)" "go run main.go project list -p" "# Projects"
run_test "project list (json)" "go run main.go project list -j" "\"id\""
# Note: project list doesn't support team filter in the API
# run_test "project list (with team filter)" "go run main.go project list --team $team_key" 
run_test "project list (state filter)" "go run main.go project list --state started"
run_test "project list (time filter)" "go run main.go project list --newer-than 1_month_ago"

# Get first project ID for project get test
project_output=$(go run main.go project list 2>/dev/null || true)
project_id=$(get_first_id "$project_output")
if [ -n "$project_id" ]; then
    run_test "project get" "go run main.go project get $project_id" "Project:"
    run_test "project get (plaintext)" "go run main.go project get $project_id -p" "# "
fi

# Test issue commands
echo -e "\n${YELLOW}Testing issue commands...${NC}"
run_test "issue list" "go run main.go issue list"
run_test "issue list (plaintext)" "go run main.go issue list -p" "# Issues"
run_test "issue list (json)" "go run main.go issue list -j" "\"identifier\""
run_test "issue list (assignee filter)" "go run main.go issue list --assignee me"
run_test "issue list (state filter)" "go run main.go issue list --state Todo"
run_test "issue list (team filter)" "go run main.go issue list --team $team_key"
run_test "issue list (priority filter)" "go run main.go issue list --priority 3"
run_test "issue list (time filter)" "go run main.go issue list --newer-than 2_weeks_ago"
run_test "issue list (sort by updated)" "go run main.go issue list --sort updated"

# Get first issue ID for additional tests
issue_output=$(go run main.go issue list --limit 5 2>/dev/null || true)
issue_id=$(echo "$issue_output" | grep -E -o '[A-Z]+-[0-9]+' | head -1)
if [ -n "$issue_id" ]; then
    run_test "issue search (default)" "go run main.go issue search $issue_id"
    run_test "issue search (json)" "go run main.go issue search $issue_id -j" "$issue_id"
    run_test "issue search (plaintext)" "go run main.go issue search $issue_id -p" "# Search Results"
    run_test "issue get" "go run main.go issue get $issue_id"
    run_test "issue get (plaintext)" "go run main.go issue get $issue_id -p" "# $issue_id"
    
    # Test comment list for this issue
    echo -e "\n${YELLOW}Testing comment commands...${NC}"
    run_test "comment list" "go run main.go comment list $issue_id"
    run_test "comment list (plaintext)" "go run main.go comment list $issue_id -p"

    # Test attachment list for this issue
    echo -e "\n${YELLOW}Testing attachment commands...${NC}"
    run_test "attachment list" "go run main.go attachment list $issue_id"
    run_test "attachment list (json)" "go run main.go attachment list $issue_id -j"
    run_test "attachment list (plaintext)" "go run main.go attachment list $issue_id -p"
    run_test "attachment list (limit)" "go run main.go attachment list $issue_id --limit 10"
    run_test "attachment list (sort)" "go run main.go attachment list $issue_id --sort created"
fi

# Test issue link command (help/error handling only - no actual linking)
echo -e "\n${YELLOW}Testing issue link command...${NC}"
run_test "issue link help" "go run main.go issue link --help" "blocks"
run_test "issue link help" "go run main.go issue link --help" "parent-of"

# Test error handling for link command
set +e
output=$(go run main.go issue link SAME-123 SAME-123 --type blocks 2>&1)
if echo "$output" | grep -q "Cannot link an issue to itself"; then
    echo -e "issue link (self-referential check): ${GREEN}PASS${NC}"
    TESTS_RUN=$((TESTS_RUN + 1))
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "issue link (self-referential check): ${RED}FAIL${NC}"
    TESTS_RUN=$((TESTS_RUN + 1))
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

output=$(go run main.go issue link A-1 B-2 --type invalid-type 2>&1)
if echo "$output" | grep -q "Invalid type"; then
    echo -e "issue link (invalid type check): ${GREEN}PASS${NC}"
    TESTS_RUN=$((TESTS_RUN + 1))
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "issue link (invalid type check): ${RED}FAIL${NC}"
    TESTS_RUN=$((TESTS_RUN + 1))
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
set -e

# Test cycle / label / state commands (Tier 2)
echo -e "\n${YELLOW}Testing cycle, label, and state commands...${NC}"
run_test "label list" "go run main.go label list --limit 5"
run_test "label list (json)" "go run main.go label list --limit 5 -j"
run_test "label list (plaintext)" "go run main.go label list --limit 5 -p" "# Labels"
if [ -n "$team_key" ]; then
    run_test "state list" "go run main.go state list $team_key"
    run_test "state list (json)" "go run main.go state list $team_key -j"
    run_test "cycle list (team filter)" "go run main.go cycle list --team $team_key"
fi

# Test document commands (Tier 2)
echo -e "\n${YELLOW}Testing document commands...${NC}"
run_test "document list" "go run main.go document list --limit 5"
run_test "document list (json)" "go run main.go document list --limit 5 -j"
run_test "document list (plaintext)" "go run main.go document list --limit 5 -p"
run_test "document search" "go run main.go document search test --limit 5"
run_test "document search (json)" "go run main.go document search test --limit 5 -j"

# Test org commands (Tier 3)
echo -e "\n${YELLOW}Testing org commands...${NC}"
run_test "org get" "go run main.go org get"
run_test "org get (json)" "go run main.go org get -j" "\"urlKey\""
run_test "org get (plaintext)" "go run main.go org get -p" "# "
run_test "org invite list" "go run main.go org invite list --limit 5"

# Test favorite, view, notification, webhook, template commands (Tier 3 - PR 4)
echo -e "\n${YELLOW}Testing favorite commands...${NC}"
run_test "favorite list" "go run main.go favorite list --limit 5"
run_test "favorite list (json)" "go run main.go favorite list --limit 5 -j"

echo -e "\n${YELLOW}Testing view commands...${NC}"
run_test "view list" "go run main.go view list --limit 5"
run_test "view list (json)" "go run main.go view list --limit 5 -j"

echo -e "\n${YELLOW}Testing notification commands...${NC}"
run_test "notification list" "go run main.go notification list --limit 5"
run_test "notification list (json)" "go run main.go notification list --limit 5 -j" "unreadCount"

echo -e "\n${YELLOW}Testing webhook commands...${NC}"
run_test "webhook list" "go run main.go webhook list --limit 5"
run_test "webhook list (json)" "go run main.go webhook list --limit 5 -j"

echo -e "\n${YELLOW}Testing template commands...${NC}"
run_test "template list" "go run main.go template list"
run_test "template list (json)" "go run main.go template list -j"
run_test "template list (type filter)" "go run main.go template list --type issue"

# Test customer/CRM commands (Tier 4a - PR 5a). Releases deferred (Business plan).
echo -e "\n${YELLOW}Testing customer commands...${NC}"
run_test "customer list" "go run main.go customer list --limit 5"
run_test "customer list (json)" "go run main.go customer list --limit 5 -j"
run_test "customer status list" "go run main.go customer status list"
run_test "customer tier list" "go run main.go customer tier list"
run_test "customer need list" "go run main.go customer need list --limit 5"
run_test "customer need list (json)" "go run main.go customer need list --limit 5 -j"

# Test emoji/triage/git/schedule commands (Tier 4b - PR 5b).
# Triage is read-only and git is write-only (create/update/delete): triage
# writes need the Business plan, and the git automation query is not exposed.
echo -e "\n${YELLOW}Testing emoji commands...${NC}"
run_test "emoji list" "go run main.go emoji list --limit 5"
run_test "emoji list (json)" "go run main.go emoji list --limit 5 -j"

echo -e "\n${YELLOW}Testing triage commands...${NC}"
run_test "triage list" "go run main.go triage list --limit 5"
run_test "triage list (json)" "go run main.go triage list --limit 5 -j"

echo -e "\n${YELLOW}Testing schedule commands...${NC}"
run_test "schedule list" "go run main.go schedule list --limit 5"
run_test "schedule list (json)" "go run main.go schedule list --limit 5 -j"

# Test attachment link commands (Tier 4c - PR 5c). OAuth apps and the Agents API
# are deferred: oauth-app needs the oauth:create scope (personal keys can't hold
# it) and agent writes need an agent-app actor.
echo -e "\n${YELLOW}Testing attachment link commands...${NC}"
# Unknown provider must exit non-zero and list the valid providers.
set +e
output=$(go run main.go attachment link SD-1 https://example.com --provider bogus 2>&1)
if echo "$output" | grep -q "valid providers are"; then
    echo -e "attachment link (invalid provider check): ${GREEN}PASS${NC}"
    TESTS_RUN=$((TESTS_RUN + 1))
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "attachment link (invalid provider check): ${RED}FAIL${NC}"
    TESTS_RUN=$((TESTS_RUN + 1))
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
set -e

# Test help commands
echo -e "\n${YELLOW}Testing help commands...${NC}"
run_test "help" "go run main.go --help" "Usage:"
run_test "issue help" "go run main.go issue --help" "Available Commands:"
run_test "project help" "go run main.go project --help" "Available Commands:"
run_test "team help" "go run main.go team --help" "Available Commands:"
run_test "user help" "go run main.go user --help" "Available Commands:"
run_test "cycle help" "go run main.go cycle --help" "Available Commands:"
run_test "label help" "go run main.go label --help" "Available Commands:"
run_test "state help" "go run main.go state --help" "Available Commands:"
run_test "document help" "go run main.go document --help" "Available Commands:"
run_test "comment help" "go run main.go comment --help" "resolve"
run_test "org help" "go run main.go org --help" "Available Commands:"
run_test "team help (set-role)" "go run main.go team --help" "set-role"
run_test "user help (suspend)" "go run main.go user --help" "suspend"
run_test "favorite help" "go run main.go favorite --help" "Available Commands:"
run_test "view help" "go run main.go view --help" "Available Commands:"
run_test "notification help" "go run main.go notification --help" "Available Commands:"
run_test "webhook help (rotate-secret)" "go run main.go webhook --help" "rotate-secret"
run_test "template help" "go run main.go template --help" "Available Commands:"
run_test "issue create help (template)" "go run main.go issue create --help" "template"
run_test "customer help (merge/need)" "go run main.go customer --help" "merge"
run_test "customer need help" "go run main.go customer need --help" "Available Commands:"
run_test "emoji help" "go run main.go emoji --help" "Available Commands:"
run_test "emoji create help (file/url)" "go run main.go emoji create --help" "file"
run_test "triage help" "go run main.go triage --help" "Available Commands:"
run_test "git help (state/target-branch)" "go run main.go git --help" "target-branch"
run_test "git state help" "go run main.go git state --help" "Available Commands:"
run_test "schedule help (upsert/refresh)" "go run main.go schedule --help" "upsert-external"
run_test "attachment help (link/sync-to-slack)" "go run main.go attachment --help" "sync-to-slack"
run_test "attachment link help (provider)" "go run main.go attachment link --help" "provider"

# Test Tier 5 cross-cutting operations (PR 6)
echo -e "\n${YELLOW}Testing Tier 5 commands...${NC}"
run_test "rate-limit" "go run main.go rate-limit" "requests"
run_test "rate-limit (json)" "go run main.go rate-limit -j" "remainingAmount"
run_test "audit list" "go run main.go audit list --limit 3"
run_test "audit types" "go run main.go audit types" "login"
run_test "audit help (list/types)" "go run main.go audit --help" "types"
run_test "search help (projects/semantic)" "go run main.go search --help" "semantic"
run_test "search projects" "go run main.go search projects email --limit 3"
run_test "search semantic" "go run main.go search semantic test --limit 3"
run_test "sla list (team)" "go run main.go sla list --team $team_key"
run_test "link help (add/update/remove)" "go run main.go link --help" "remove"
run_test "user external list" "go run main.go user external list --limit 3"
run_test "user external help" "go run main.go user external --help" "Available Commands:"
run_test "view prefs help" "go run main.go view prefs --help" "Available Commands:"
run_test "issue relate help (update)" "go run main.go issue relate --help" "update"
run_test "user settings update help (privacy-legal)" "go run main.go user settings update --help" "privacy-legal"

# Test unknown command handling
echo -e "\n${YELLOW}Testing error handling...${NC}"
# This should fail but gracefully
set +e
output=$(go run main.go nonexistent-command 2>&1)
if echo "$output" | grep -q "unknown command"; then
    echo -e "Unknown command handling: ${GREEN}PASS${NC}"
    TESTS_RUN=$((TESTS_RUN + 1))
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "Unknown command handling: ${RED}FAIL${NC}"
    TESTS_RUN=$((TESTS_RUN + 1))
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
set -e

# Summary
echo -e "\n================================"
echo "Test Summary:"
echo "  Total tests: $TESTS_RUN"
echo -e "  Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "  Failed: ${RED}$TESTS_FAILED${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "\n${GREEN}✅ All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}❌ Some tests failed!${NC}"
    exit 1
fi
