# Rename linctl to lincli Design

**Date:** 2025-11-14
**Status:** Approved
**Priority:** Must complete before GraphQL migration

## Problem Statement

The current project name is `linctl` (forked from dorkitude/linctl). We want to rename it to `lincli` for this fork, which requires updating:
- Binary name
- Go module path (to reflect shanedolley ownership)
- Configuration file paths
- All documentation
- Import statements

## Goals

1. Rename binary from `linctl` to `lincli`
2. Update Go module path from `github.com/dorkitude/linctl` to `github.com/shanedolley/lincli`
3. Maintain user configuration compatibility (migrate old config paths)
4. Update all documentation
5. Clean, committable change that doesn't break functionality

## Changes Required

### 1. Go Module & Imports

**Files to update:**
- `go.mod` - Module path declaration
- All `*.go` files with imports (estimated 15-20 files)

**Pattern:**
```go
// Old
import "github.com/dorkitude/linctl/pkg/api"

// New
import "github.com/shanedolley/lincli/pkg/api"
```

### 2. Binary Name

**Files to update:**
- `Makefile` - `BINARY_NAME` variable
- `.gitignore` - Binary name reference
- All shell scripts referencing the binary

**Pattern:**
```makefile
# Old
BINARY_NAME=linctl

# New
BINARY_NAME=lincli
```

### 3. Configuration Files

**User-facing paths:**
- `~/.linctl.yaml` → `~/.lincli.yaml`
- `~/.linctl-auth.json` → `~/.lincli-auth.json`

**Code locations:**
- `cmd/root.go` - Config file name
- `pkg/auth/auth.go` - Auth file name

**Strategy:** Add migration code that checks for old paths and copies to new paths on first run.

### 4. Documentation

**Files to update:**
- `README.md` - All command examples and references
- `CLAUDE.md` - Architecture documentation
- `CONTRIBUTING.md` - Release instructions
- `docs/plans/*.md` - Any references to linctl

**Estimated changes:** 100+ occurrences of "linctl" across docs

### 5. Homebrew Formula

**Files to update:**
- `Formula/linctl.rb` - Rename to `lincli.rb` and update contents

**Note:** Since this is a fork, Homebrew formula points to dorkitude repo. We'll update it but won't publish until we have our own tap.

### 6. Repository References

**Files to update:**
- `README.md` - GitHub URLs
- `CONTRIBUTING.md` - Repository references
- Any other hardcoded repository URLs

**Pattern:**
```
# Old
https://github.com/dorkitude/linctl

# New
https://github.com/shanedolley/lincli
```

## Implementation Plan

### Phase 1: Go Code Changes
1. Update `go.mod` module path
2. Find and replace all imports: `github.com/dorkitude/linctl` → `github.com/shanedolley/lincli`
3. Run `go mod tidy`
4. Verify: `go build` succeeds

### Phase 2: Binary & Build System
1. Update `Makefile` - `BINARY_NAME` variable
2. Update `.gitignore` - binary name
3. Update `smoke_test.sh` - binary references
4. Verify: `make build` produces `lincli` binary
5. Verify: `make test` passes

### Phase 3: Configuration Migration
1. Update config file names in `cmd/root.go`
2. Add migration logic in `cmd/root.go` init:
   ```go
   // Check for old config, copy to new location
   if oldConfigExists() && !newConfigExists() {
       copyConfig(old, new)
   }
   ```
3. Update auth file name in `pkg/auth/auth.go`
4. Add migration logic in `pkg/auth/auth.go`
5. Test: Create old config files, verify migration

### Phase 4: Documentation Update
1. Update `README.md`:
   - Global replace `linctl` → `lincli`
   - Update repository URLs
   - Update Homebrew tap instructions
2. Update `CLAUDE.md`:
   - Replace binary name references
   - Update module path in examples
3. Update `CONTRIBUTING.md`:
   - Update release instructions
   - Update repository URLs
4. Update `docs/plans/*.md` if needed

### Phase 5: Homebrew Formula
1. Rename `Formula/linctl.rb` → `Formula/lincli.rb`
2. Update class name inside formula
3. Update repository URLs
4. Update binary name references

### Phase 6: Final Verification
1. Clean build: `make clean && make build`
2. Run smoke tests: `make test`
3. Test auth flow: `./lincli auth status`
4. Test config migration: Create old files, verify copy
5. Verify documentation examples work

## Configuration Migration Details

### Migration Code Pattern

Add to `cmd/root.go`:
```go
func migrateOldConfig() {
    home, _ := os.UserHomeDir()
    oldConfig := filepath.Join(home, ".linctl.yaml")
    newConfig := filepath.Join(home, ".lincli.yaml")

    if _, err := os.Stat(oldConfig); err == nil {
        if _, err := os.Stat(newConfig); os.IsNotExist(err) {
            // Copy old to new, don't delete old (safer)
            data, _ := os.ReadFile(oldConfig)
            os.WriteFile(newConfig, data, 0600)
        }
    }
}
```

Add to `pkg/auth/auth.go`:
```go
func migrateOldAuth() {
    home, _ := os.UserHomeDir()
    oldAuth := filepath.Join(home, ".linctl-auth.json")
    newAuth := filepath.Join(home, ".lincli-auth.json")

    if _, err := os.Stat(oldAuth); err == nil {
        if _, err := os.Stat(newAuth); os.IsNotExist(err) {
            data, _ := os.ReadFile(oldAuth)
            os.WriteFile(newAuth, data, 0600)
        }
    }
}
```

## Success Criteria

- [ ] `go build` succeeds with new module path
- [ ] Binary is named `lincli`
- [ ] `make test` passes
- [ ] All imports updated
- [ ] Config migration code works
- [ ] Auth migration code works
- [ ] Documentation references updated
- [ ] No references to old name in codebase
- [ ] README examples use `lincli`
- [ ] Homebrew formula updated

## Rollback Plan

This is a single atomic commit. If issues arise:
```bash
git revert HEAD
go mod tidy
make clean && make build
```

## Estimated Timeline

- Phase 1-2: 30 minutes (Go code & build)
- Phase 3: 20 minutes (config migration)
- Phase 4: 30 minutes (documentation)
- Phase 5: 10 minutes (Homebrew formula)
- Phase 6: 20 minutes (verification)

**Total:** ~2 hours

## Post-Rename Tasks

After completing the rename:
1. Push to your fork: `git push origin master`
2. Update GitHub repository name (optional): `linctl` → `lincli`
3. Consider creating your own Homebrew tap: `shanedolley/homebrew-lincli`
4. Proceed with GraphQL migration design

## Notes

- Keep old config files after migration (safer, users can delete manually)
- This fork is independent from dorkitude/linctl - no upstream sync planned
- Homebrew formula won't work until published to a tap (not critical for personal use)
