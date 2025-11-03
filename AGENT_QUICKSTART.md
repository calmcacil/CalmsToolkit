# TUI Uplift - Agent Quick Start Guide

**Your Mission:** Implement one phase of the TUI uplift using Git worktrees for isolated, parallel development.

## 🚀 Quick Setup (5 minutes)

### Step 1: Understand Your Role

```bash
# List all available worktrees
cd /Users/samanthamyers/Development/CalmsToolkit
git worktree list
```

**You are assigned to ONE of these:**
- `phase1-foundation` - Shared components & app infrastructure
- `phase2-mediarequests` - Media Requests tool
- `phase2-streams` - Streams tool  
- `phase2-calendar` - Calendar tool
- `phase2-queue` - Queue Remediation tool
- `phase3-arrfeed` - ARR Feed tool
- `phase3-polish` - Integration, testing, polish

### Step 2: Enter Your Worktree

```bash
cd .worktrees/YOUR_PHASE_NAME/

# Verify correct branch
git branch

# Verify correct starting point (should show: 212ddef)
git log -1 --oneline

# Check what you should work on
git status
```

### Step 3: Create Your Feature Branch

```bash
# Create a descriptive feature branch within your phase
git checkout -b feature/your-feature-name

# Example:
# git checkout -b feature/mediarequests-search-component
# git checkout -b feature/foundation-app-bootstrap
```

### Step 4: Start Development

Work in your worktree without worrying about other agents! Your changes are isolated.

```bash
# Write code
# vim internal/tools/mediarequests/model.go

# Format with gofmt
go fmt ./...

# Test frequently
go test ./...

# When ready, commit
git add .
git commit -m "feat: implement feature description"
```

### Step 5: Push & Create PR

```bash
# Push your feature branch
git push origin feature/your-feature-name

# The system will show you a GitHub URL
# Open PR to tui/integration (NOT main)
# Include these details:
# - What you implemented
# - Tests you added
# - Any dependencies added to go.mod
```

## 📋 Phase-Specific Guidance

### Phase 1: Foundation (Shared Components)

**Your Worktree:** `.worktrees/phase1-foundation/`

**Scope:** YOU OWN these directories:
- `cmd/calms-toolkit/` - Entry point
- `internal/app/` - Main app model
- `internal/components/` - Shared UI components
- `internal/config/` - Config loading
- `internal/api/` - HTTP client
- `pkg/colors/` - Color system

**What to Build:**

1. **Project Structure**
   ```bash
   mkdir -p cmd/calms-toolkit
   mkdir -p internal/app internal/components internal/config internal/api
   mkdir -p pkg/colors
   ```

2. **Extract Shared Components** from existing tools:
   - HTTP client logic
   - ANSI color constants
   - Configuration parsing

3. **Bootstrap Bubble Tea App**
   - Main entry point
   - Central model with tab navigation
   - Window size handling

4. **Write Tests** for each component

5. **Don't Touch:** `internal/tools/` (those are for Phase 2)

**Success Criteria:**
- ✅ `cmd/calms-toolkit/main.go` compiles
- ✅ Basic tab navigation works
- ✅ Config loading from env/file works
- ✅ Unit tests for config, colors, HTTP client pass
- ✅ No files outside your scope modified

---

### Phase 2a: Media Requests Tool

**Your Worktree:** `.worktrees/phase2-mediarequests/`

**Scope:** YOU OWN this directory:
- `internal/tools/mediarequests/` - All files here

**Files to Create:**
```
internal/tools/mediarequests/
├── model.go       # State: search, select, confirm
├── update.go      # Handle Bubble Tea messages
├── view.go        # Render UI
├── api.go         # API calls to Overseerr
├── types.go       # Custom types
└── model_test.go  # Tests
```

**What to Implement:**

1. **Model Structure**
   ```go
   type Model struct {
       step      string  // "search", "select", "confirm"
       query     string
       results   []SearchResult
       selected  int
       loading   bool
       error     string
   }
   ```

2. **State Machine**
   - User enters search query
   - App fetches results
   - User selects result
   - User confirms request
   - Request submitted

3. **Use Foundation Components**
   - Config from `internal/config/`
   - HTTP client from `internal/api/`
   - Colors from `pkg/colors/`

4. **Write Tests**
   - Test state transitions
   - Mock API responses
   - Test error handling

**Success Criteria:**
- ✅ Compiles without errors
- ✅ All unit tests pass
- ✅ Only touches `internal/tools/mediarequests/`
- ✅ No modifications to foundation files
- ✅ Uses foundation components properly
- ✅ gofmt clean

---

### Phase 2b, 2c, 2d: Other Tools (Streams, Calendar, Queue)

**Same pattern as Phase 2a:**
- Create isolated directory in `internal/tools/`
- Implement model.go, update.go, view.go, api.go
- Write comprehensive tests
- Don't modify files outside your directory
- Don't modify foundation (that's Phase 1's job)

---

### Phase 3a: ARR Feed Tool

**Your Worktree:** `.worktrees/phase3-arrfeed/`

**Requirements:**
- Phase 1 must be merged
- Phase 2 must be mostly merged

**Same pattern:**
- Create `internal/tools/arrfeed/`
- Implement tool following Bubble Tea patterns
- Tests for all functionality

---

### Phase 3b: Polish & Testing

**Your Worktree:** `.worktrees/phase3-polish/`

**Scope:** YOU OWN:
- `*_test.go` files for integration
- `Makefile` updates
- `docs/` updates
- Minor fixes across the app

**What to Do:**

1. **Integration Testing**
   - Test full workflows
   - Test tool switching
   - Test error conditions

2. **Performance Testing**
   - Check for memory leaks
   - Test with large datasets
   - Profile hot paths

3. **UX Polish**
   - Keyboard navigation improvements
   - Color refinements
   - Help text / keybindings display

4. **Documentation**
   - Update README
   - Add keyboard shortcuts guide
   - Add troubleshooting guide

---

## 🔄 Common Workflows

### I Need Latest Foundation Updates

```bash
cd .worktrees/YOUR_PHASE/
git fetch origin
git merge origin/tui/integration --no-edit

# Resolve conflicts if any (should be minimal)
# Then continue working
```

### I Accidentally Modified Files Outside My Scope

```bash
# Check what you changed
git status

# Revert files outside your scope
git checkout -- path/to/file/outside/scope

# Keep your in-scope changes
git add path/to/file/in/scope
git commit -m "revert out-of-scope changes"
```

### I Need to Reset My Worktree

```bash
# Save any uncommitted work
git stash

# Reset to clean state
git checkout tui/YOUR_PHASE
git pull origin tui/YOUR_PHASE
git merge origin/tui/integration --no-edit

# Restore your work
git stash pop
```

### I Want to Test How My Work Integrates

```bash
# From main repo directory
cd /Users/samanthamyers/Development/CalmsToolkit

# Build the app
go build -o bin/calms-toolkit ./cmd/calms-toolkit

# Run it
./bin/calms-toolkit

# Run all tests
go test ./...
```

---

## ✅ Before You Push

### Checklist

- [ ] Ran `go fmt ./...` on your changes
- [ ] Ran `go test ./...` - all tests pass
- [ ] Only modified files in your scope
- [ ] Didn't break any other tests
- [ ] Committed with clear, imperative messages
- [ ] Pulled latest from origin (no conflicts)
- [ ] Verified no unintended files in your commit

### Example Good Commit Message

```
feat: implement media request search UI component

- Add search input with autocomplete
- Implement result pagination
- Add keyboard navigation
- Handle API errors gracefully

Tests:
- Table-driven tests for state transitions
- Mock API responses
- Error handling verification
```

### Example Bad Commit Messages (Don't Do These)

```
❌ Update stuff
❌ Fix things
❌ WIP
❌ asdf
❌ Fixed a bug (what bug?)
```

---

## 🆘 Getting Help

### Issue: "My branch is behind integration"

```bash
# Pull latest
git fetch origin
git merge origin/tui/integration
```

### Issue: "Merge conflict in shared file"

This shouldn't happen if scopes are respected. If it does:
1. Check you're in the right worktree
2. Compare conflict markers with your phase scope
3. If in your scope: resolve normally
4. If NOT in your scope: restore the file as-is (they win)
5. Open an issue linking your worktree

### Issue: "I modified a file I shouldn't have"

```bash
# Option 1: Unstage it
git reset path/to/wrong/file

# Option 2: Revert it
git checkout -- path/to/wrong/file

# Option 3: Remove from commit
git commit --amend -- path/to/wrong/file
```

### Issue: "Tests fail but I didn't touch that code"

```bash
# Pull latest foundation
git fetch origin
git merge origin/tui/integration

# Re-run tests
go test ./...

# If still failing, it's a real problem
# If now passing, just needed foundation updates
```

---

## 📊 Phase Dependencies

```
Phase 1: Foundation
    ↓ (blocks Phase 2)
Phase 2a: Media Requests
Phase 2b: Streams  
Phase 2c: Calendar
Phase 2d: Queue
    ↓ (all must complete before Phase 3a)
Phase 3a: ARR Feed
    ↓ (blocks Phase 3b)
Phase 3b: Polish
    ↓
Main branch
```

**What this means for you:**
- Phase 1 agents: START IMMEDIATELY (no blockers)
- Phase 2 agents: WAIT for Phase 1 merge, then START
- Phase 3a agents: WAIT for Phase 2 merges
- Phase 3b agents: WAIT for Phase 3a merge

---

## 🎯 Success = Isolated Parallel Development

When everyone follows this process:
- ✅ No merge conflicts between agents
- ✅ No blocked PRs waiting for others
- ✅ Clear code ownership and accountability
- ✅ Easy to review (each PR is focused)
- ✅ Easy to rollback if issues occur
- ✅ Foundation can be refined without breaking tool work

You're solving one problem in isolation. That's the entire power of this approach.

---

**Document Version:** 1.0  
**Created:** November 3, 2025
