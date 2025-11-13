# Rename linctl to lincli Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rename the forked project from `linctl` to `lincli`, including module path, binary name, config files, and all documentation.

**Architecture:** This is a mechanical rename touching ~30 files. The critical changes are: (1) Go module path from `github.com/dorkitude/linctl` to `github.com/shanedolley/lincli`, (2) binary name from `linctl` to `lincli`, (3) config file migration with backward compatibility.

**Tech Stack:** Go 1.23+, git, standard Unix tools

**Working Directory:** `.worktrees/rename-to-lincli` (isolated worktree)

---

## Phase 1: Go Module and Import Path Changes

### Task 1: Update go.mod Module Declaration

**Files:**
- Modify: `go.mod:1`

**Step 1: Update module path**

Edit the first line of `go.mod`:

```go
module github.com/shanedolley/lincli
```

**Step 2: Verify syntax**

Run: `go mod tidy`
Expected: No errors, dependencies resolve correctly

**Step 3: Commit**

```bash
git add go.mod
git commit -m "refactor: update module path to shanedolley/lincli"
```

---

### Task 2: Update Imports in main.go

**Files:**
- Modify: `main.go:6`

**Step 1: Update import statement**

```go
import (
	_ "embed"

	"github.com/shanedolley/lincli/cmd"
)
```

**Step 2: Verify it compiles**

Run: `go build`
Expected: FAIL with import errors from other files (expected at this stage)

**Step 3: Commit**

```bash
git add main.go
git commit -m "refactor: update main.go import path"
```

---

### Task 3: Update Imports in cmd/auth.go

**Files:**
- Modify: `cmd/auth.go:9-11`

**Step 1: Update import statements**

Find the import block around line 9-11 and update:

```go
import (
	"fmt"
	"os"

	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)
```

**Step 2: Commit**

```bash
git add cmd/auth.go
git commit -m "refactor: update cmd/auth.go import paths"
```

---

### Task 4: Update Imports in cmd/comment.go

**Files:**
- Modify: `cmd/comment.go:5-10`

**Step 1: Update import statements**

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)
```

**Step 2: Commit**

```bash
git add cmd/comment.go
git commit -m "refactor: update cmd/comment.go import paths"
```

---

### Task 5: Update Imports in cmd/issue.go

**Files:**
- Modify: `cmd/issue.go:9-12`

**Step 1: Update import statements**

```go
import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/shanedolley/lincli/pkg/utils"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)
```

**Step 2: Commit**

```bash
git add cmd/issue.go
git commit -m "refactor: update cmd/issue.go import paths"
```

---

### Task 6: Update Imports in cmd/project.go

**Files:**
- Modify: `cmd/project.go:5-10`

**Step 1: Update import statements**

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/shanedolley/lincli/pkg/utils"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)
```

**Step 2: Commit**

```bash
git add cmd/project.go
git commit -m "refactor: update cmd/project.go import paths"
```

---

### Task 7: Update Imports in cmd/team.go

**Files:**
- Modify: `cmd/team.go:5-10`

**Step 1: Update import statements**

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)
```

**Step 2: Commit**

```bash
git add cmd/team.go
git commit -m "refactor: update cmd/team.go import paths"
```

---

### Task 8: Update Imports in cmd/user.go

**Files:**
- Modify: `cmd/user.go:5-10`

**Step 1: Update import statements**

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
	"github.com/shanedolley/lincli/pkg/output"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)
```

**Step 2: Commit**

```bash
git add cmd/user.go
git commit -m "refactor: update cmd/user.go import paths"
```

---

### Task 9: Update Imports in pkg/auth/auth.go

**Files:**
- Modify: `pkg/auth/auth.go:12`

**Step 1: Update import statement**

```go
import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shanedolley/lincli/pkg/api"
	"github.com/fatih/color"
)
```

**Step 2: Commit**

```bash
git add pkg/auth/auth.go
git commit -m "refactor: update pkg/auth/auth.go import path"
```

---

### Task 10: Verify All Go Code Compiles

**Step 1: Run tidy**

Run: `go mod tidy`
Expected: No errors

**Step 2: Build binary**

Run: `go build -o linctl .`
Expected: SUCCESS, binary created

**Step 3: Quick smoke test**

Run: `./linctl --version`
Expected: Version output (binary name still shows old path in version, that's ok)

**Step 4: Commit verification checkpoint**

```bash
git add go.sum
git commit -m "chore: update go.sum after import path changes"
```

---

## Phase 2: Binary Name and Build System

### Task 11: Update Binary Name in Makefile

**Files:**
- Modify: `Makefile:6`

**Step 1: Update BINARY_NAME variable**

Change line 6:

```makefile
BINARY_NAME=lincli
```

**Step 2: Update LDFLAGS module path**

Change line 10:

```makefile
LDFLAGS=-ldflags "-X github.com/shanedolley/lincli/cmd.version=$(VERSION)"
```

**Step 3: Test build**

Run: `make clean && make build`
Expected: Creates `lincli` binary (not `linctl`)

**Step 4: Verify binary works**

Run: `./lincli --version`
Expected: Version output

**Step 5: Commit**

```bash
git add Makefile
git commit -m "refactor: rename binary from linctl to lincli"
```

---

### Task 12: Update Binary Name in .gitignore

**Files:**
- Modify: `.gitignore:32`

**Step 1: Update binary name**

Change line 32:

```
# Build output
lincli
dist/
```

**Step 2: Commit**

```bash
git add .gitignore
git commit -m "refactor: update .gitignore for lincli binary"
```

---

### Task 13: Update Binary References in smoke_test.sh

**Files:**
- Modify: `smoke_test.sh` (multiple lines)

**Step 1: Find and replace binary name**

Run: `sed -i.bak 's/linctl/lincli/g' smoke_test.sh && rm smoke_test.sh.bak`

**Step 2: Verify changes**

Run: `grep -n "lincli" smoke_test.sh | head -5`
Expected: Should see `lincli` references

**Step 3: Test smoke tests**

Run: `make build && ./smoke_test.sh`
Expected: 39 tests passing

**Step 4: Commit**

```bash
git add smoke_test.sh
git commit -m "refactor: update smoke_test.sh to use lincli"
```

---

## Phase 3: Configuration File Migration

### Task 14: Add Config Migration to cmd/root.go

**Files:**
- Modify: `cmd/root.go:88` (in init function)

**Step 1: Add migration function before cobra.OnInitialize**

Add this function before the `init()` function (around line 88):

```go
// migrateOldConfig copies old linctl config to new lincli location
func migrateOldConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	oldConfig := filepath.Join(home, ".linctl.yaml")
	newConfig := filepath.Join(home, ".lincli.yaml")

	// Only migrate if old exists and new doesn't
	if _, err := os.Stat(oldConfig); err == nil {
		if _, err := os.Stat(newConfig); os.IsNotExist(err) {
			data, err := os.ReadFile(oldConfig)
			if err == nil {
				_ = os.WriteFile(newConfig, data, 0600)
			}
		}
	}
}
```

**Step 2: Call migration in init**

Add at the start of `init()` function (line 88):

```go
func init() {
	migrateOldConfig()
	cobra.OnInitialize(initConfig)
	// ... rest of init
}
```

**Step 3: Add filepath import**

Add to imports at top of file:

```go
import (
	"fmt"
	"os"
	"path/filepath"  // Add this
	"strings"
	// ... rest
)
```

**Step 4: Update config file name in initConfig**

Find line 114, change:

```go
viper.SetConfigName(".lincli")
```

**Step 5: Test compilation**

Run: `make build`
Expected: SUCCESS

**Step 6: Commit**

```bash
git add cmd/root.go
git commit -m "feat: add config migration from .linctl.yaml to .lincli.yaml"
```

---

### Task 15: Add Auth File Migration to pkg/auth/auth.go

**Files:**
- Modify: `pkg/auth/auth.go:28` (before getConfigPath)

**Step 1: Add migration function**

Add before `getConfigPath()` function (around line 28):

```go
// migrateOldAuthFile copies old linctl auth to new lincli location
func migrateOldAuthFile() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	oldAuth := filepath.Join(home, ".linctl-auth.json")
	newAuth := filepath.Join(home, ".lincli-auth.json")

	// Only migrate if old exists and new doesn't
	if _, err := os.Stat(oldAuth); err == nil {
		if _, err := os.Stat(newAuth); os.IsNotExist(err) {
			data, err := os.ReadFile(oldAuth)
			if err == nil {
				_ = os.WriteFile(newAuth, data, 0600)
			}
		}
	}
}
```

**Step 2: Update getConfigPath function**

Modify `getConfigPath()` to call migration and use new filename:

```go
func getConfigPath() (string, error) {
	migrateOldAuthFile()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".lincli-auth.json"), nil
}
```

**Step 3: Test compilation**

Run: `make build`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add pkg/auth/auth.go
git commit -m "feat: add auth file migration from .linctl-auth.json to .lincli-auth.json"
```

---

## Phase 4: Documentation Updates

### Task 16: Update README.md (Part 1: Title and Intro)

**Files:**
- Modify: `README.md:1-96`

**Step 1: Global find and replace**

Run: `sed -i.bak 's/linctl/lincli/g' README.md && rm README.md.bak`

**Step 2: Update repository URLs**

Run: `sed -i.bak 's|github.com/dorkitude/linctl|github.com/shanedolley/lincli|g' README.md && rm README.md.bak`

**Step 3: Update Homebrew tap references**

Run: `sed -i.bak 's|dorkitude/linctl|shanedolley/lincli|g' README.md && rm README.md.bak`

**Step 4: Verify changes**

Run: `head -50 README.md`
Expected: Title should be "🚀 lincli - Linear CLI Tool"

**Step 5: Commit**

```bash
git add README.md
git commit -m "docs: update README.md with lincli branding"
```

---

### Task 17: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md` (entire file)

**Step 1: Update all linctl references**

Run: `sed -i.bak 's/linctl/lincli/g' CLAUDE.md && rm CLAUDE.md.bak`

**Step 2: Update module paths**

Run: `sed -i.bak 's|github.com/dorkitude/linctl|github.com/shanedolley/lincli|g' CLAUDE.md && rm CLAUDE.md.bak`

**Step 3: Update repository URLs**

Run: `sed -i.bak 's|dorkitude/linctl|shanedolley/lincli|g' CLAUDE.md && rm CLAUDE.md.bak`

**Step 4: Verify changes**

Run: `head -20 CLAUDE.md`
Expected: References should be "lincli"

**Step 5: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with lincli references"
```

---

### Task 18: Update CONTRIBUTING.md

**Files:**
- Modify: `CONTRIBUTING.md` (entire file)

**Step 1: Update all references**

Run: `sed -i.bak 's/linctl/lincli/g' CONTRIBUTING.md && rm CONTRIBUTING.md.bak`

**Step 2: Update repository URLs**

Run: `sed -i.bak 's|dorkitude/linctl|shanedolley/lincli|g' CONTRIBUTING.md && rm CONTRIBUTING.md.bak`

**Step 3: Update Homebrew tap**

Run: `sed -i.bak 's|homebrew-linctl|homebrew-lincli|g' CONTRIBUTING.md && rm CONTRIBUTING.md.bak`

**Step 4: Commit**

```bash
git add CONTRIBUTING.md
git commit -m "docs: update CONTRIBUTING.md with lincli references"
```

---

### Task 19: Update master_api_ref.md

**Files:**
- Modify: `master_api_ref.md` (if it contains linctl references)

**Step 1: Check for references**

Run: `grep -c "linctl" master_api_ref.md || echo "no references"`

**Step 2: Update if needed**

If references found:
Run: `sed -i.bak 's/linctl/lincli/g' master_api_ref.md && rm master_api_ref.md.bak`

**Step 3: Commit if changed**

```bash
git add master_api_ref.md
git commit -m "docs: update master_api_ref.md with lincli references"
```

---

### Task 20: Update Design Document References

**Files:**
- Modify: `docs/plans/2025-11-14-graphql-codegen-migration-design.md`

**Step 1: Update linctl references in GraphQL design doc**

Run: `sed -i.bak 's/linctl/lincli/g' docs/plans/2025-11-14-graphql-codegen-migration-design.md && rm docs/plans/2025-11-14-graphql-codegen-migration-design.md.bak`

**Step 2: Update module path**

Run: `sed -i.bak 's|github.com/dorkitude/linctl|github.com/shanedolley/lincli|g' docs/plans/2025-11-14-graphql-codegen-migration-design.md && rm docs/plans/2025-11-14-graphql-codegen-migration-design.md.bak`

**Step 3: Commit**

```bash
git add docs/plans/2025-11-14-graphql-codegen-migration-design.md
git commit -m "docs: update GraphQL design doc with lincli references"
```

---

## Phase 5: Homebrew Formula

### Task 21: Rename and Update Homebrew Formula

**Files:**
- Rename: `Formula/linctl.rb` → `Formula/lincli.rb`
- Modify: `Formula/lincli.rb` (entire file)

**Step 1: Rename file**

Run: `git mv Formula/linctl.rb Formula/lincli.rb`

**Step 2: Update class name**

Run: `sed -i.bak 's/class Linctl < Formula/class Lincli < Formula/' Formula/lincli.rb && rm Formula/lincli.rb.bak`

**Step 3: Update URLs**

Run: `sed -i.bak 's|dorkitude/linctl|shanedolley/lincli|g' Formula/lincli.rb && rm Formula/lincli.rb.bak`

**Step 4: Update binary name**

Run: `sed -i.bak 's/linctl/lincli/g' Formula/lincli.rb && rm Formula/lincli.rb.bak`

**Step 5: Verify formula syntax**

Run: `cat Formula/lincli.rb | head -20`
Expected: Class name should be "Lincli"

**Step 6: Commit**

```bash
git add Formula/
git commit -m "refactor: rename and update Homebrew formula to lincli"
```

---

## Phase 6: Final Verification

### Task 22: Clean Build and Full Test Suite

**Step 1: Clean all artifacts**

Run: `make clean`
Expected: Removes old binaries

**Step 2: Fresh build**

Run: `make build`
Expected: Creates `lincli` binary, ~13MB

**Step 3: Verify binary name**

Run: `ls -lh lincli`
Expected: File exists and is executable

**Step 4: Run full smoke tests**

Run: `make test`
Expected: 39 tests passing, 0 failures

**Step 5: Test version output**

Run: `./lincli --version`
Expected: Shows version

**Step 6: Test help**

Run: `./lincli --help`
Expected: Shows "lincli" in help text

---

### Task 23: Verify Config Migration Works

**Step 1: Create old config file**

Run:
```bash
mkdir -p /tmp/lincli-test
echo "output: table" > /tmp/lincli-test/.linctl.yaml
HOME=/tmp/lincli-test ./lincli --version
```

**Step 2: Check migration happened**

Run: `ls -la /tmp/lincli-test/.lincli.yaml`
Expected: File exists (migrated from old name)

**Step 3: Cleanup test**

Run: `rm -rf /tmp/lincli-test`

---

### Task 24: Verify No Remaining Old References

**Step 1: Search for linctl in code**

Run: `grep -r "linctl" --include="*.go" --include="*.md" --include="Makefile" --include="*.sh" . | grep -v ".worktrees" | grep -v ".git" | grep -v "Binary file"`

Expected: Only find references in:
- This plan document
- Rename design document
- Migration code comments (if any)

**Step 2: Search for old module path**

Run: `grep -r "github.com/dorkitude/linctl" --include="*.go" . | grep -v ".worktrees" | grep -v ".git"`

Expected: No results

**Step 3: Document verification**

Create file: `docs/rename-verification.txt`

```text
Rename Verification - 2025-11-14
=================================

Binary Name: lincli ✓
Module Path: github.com/shanedolley/lincli ✓
All Go Files: Updated ✓
Documentation: Updated ✓
Smoke Tests: 39 passing ✓
Config Migration: Working ✓

All references to linctl have been successfully renamed to lincli.
```

**Step 4: Commit verification doc**

```bash
git add docs/rename-verification.txt
git commit -m "docs: add rename verification checklist"
```

---

### Task 25: Create Final Summary Commit

**Step 1: Review all changes**

Run: `git log --oneline origin/master..HEAD`
Expected: Should see ~25 commits for the rename

**Step 2: Verify working directory is clean**

Run: `git status`
Expected: "nothing to commit, working tree clean"

**Step 3: Tag the completion**

Run: `git tag -a rename-to-lincli-complete -m "Completed rename from linctl to lincli"`

**Step 4: Show summary**

Run: `git diff --stat origin/master..HEAD`
Expected: Shows all modified files

---

## Success Criteria Checklist

After completing all tasks, verify:

- [ ] Binary is named `lincli` (not `linctl`)
- [ ] `make build` succeeds
- [ ] `make test` shows 39 passing tests
- [ ] Go module path is `github.com/shanedolley/lincli`
- [ ] All imports updated in 8 Go files
- [ ] Config files use `.lincli.yaml` and `.lincli-auth.json`
- [ ] Migration code handles old config files
- [ ] README.md uses `lincli` in all examples
- [ ] CLAUDE.md updated with new names
- [ ] CONTRIBUTING.md updated
- [ ] Homebrew formula renamed and updated
- [ ] No grep results for old module path in Go files
- [ ] `git status` shows clean working tree

## Post-Rename Next Steps

After merging this branch:

1. Update GitHub repository name (optional): `linear-cli` → `lincli`
2. Create Homebrew tap: `shanedolley/homebrew-lincli`
3. Proceed with GraphQL code generation migration
4. Update any external documentation or links

## Notes for Engineer

- **Config migration is non-destructive**: Old files are copied, not moved
- **All sed commands use .bak**: Safe pattern with backup creation
- **Each task is independently committable**: Small, focused commits
- **Tests run after each major phase**: Catches issues early
- **Working in isolated worktree**: Main branch stays clean
