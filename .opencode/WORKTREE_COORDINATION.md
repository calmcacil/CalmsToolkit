# TUI Uplift Worktree Coordination Guide

**Last Updated:** November 3, 2025  
**Base Branch:** `tui/integration` (commit: 212ddef - includes queue remediation functionality)  
**Framework:** Bubble Tea v1.3.10 + Bubbles

## Worktree Structure Overview

```
.worktrees/
├── phase1-foundation/        (Agent: Shared Components Lead)
│   ├── Branch: tui/phase1-foundation
│   ├── Purpose: Foundation infrastructure & shared components
│   ├── Scope: cmd/, internal/{app,components,config,api}/, pkg/colors/
│   └── Files: Base app model, config loading, HTTP client, color system
│
├── phase2-mediarequests/     (Agent: Media Requests Developer)
│   ├── Branch: tui/phase2-mediarequests
│   ├── Purpose: Media Requests tool TUI implementation
│   ├── Scope: internal/tools/mediarequests/
│   └── Files: model.go, update.go, view.go, api.go, tests
│
├── phase2-streams/           (Agent: Streams Developer)
│   ├── Branch: tui/phase2-streams
│   ├── Purpose: Streams tool TUI implementation
│   ├── Scope: internal/tools/streams/
│   └── Files: model.go, update.go, view.go, api.go, tests
│
├── phase2-calendar/          (Agent: Calendar Developer)
│   ├── Branch: tui/phase2-calendar
│   ├── Purpose: Calendar tool TUI implementation
│   ├── Scope: internal/tools/calendar/
│   └── Files: model.go, update.go, view.go, api.go, tests
│
├── phase2-queue/             (Agent: Queue Remediation Developer)
│   ├── Branch: tui/phase2-queue
│   ├── Purpose: Queue Remediation tool TUI implementation
│   ├── Scope: internal/tools/queue/
│   └── Files: model.go, update.go, view.go, api.go, tests
│
├── phase3-arrfeed/           (Agent: ARR Feed Developer)
│   ├── Branch: tui/phase3-arrfeed
│   ├── Purpose: ARR Feed tool TUI implementation
│   ├── Scope: internal/tools/arrfeed/
│   └── Files: model.go, update.go, view.go, api.go, tests
│
├── phase3-polish/            (Agent: Polish & Testing Lead)
│   ├── Branch: tui/phase3-polish
│   ├── Purpose: Integration testing, performance optimization, UX polish
│   ├── Scope: Cross-tool integration, e2e tests, documentation
│   └── Files: Tests, docs, minor fixes across tools
│
└── integration/              (Integration Hub - Merge Point)
    ├── Branch: tui/integration
    ├── Purpose: Central coordination point for merging all TUI work
    ├── Role: Accept PRs from all phase branches
    └── Workflow: Phase work → PR → Code Review → Merge to integration
```

## File Organization Strategy

The target structure after all phases are merged:

```
calms-toolkit/
├── cmd/calms-toolkit/
│   └── main.go                    # TUI Application entry point
│
├── internal/
│   ├── app/
│   │   ├── model.go               # Central state model
│   │   ├── update.go              # Update logic
│   │   ├── view.go                # View rendering
│   │   ├── keymap.go              # Key bindings
│   │   └── *_test.go              # App tests
│   │
│   ├── components/                # Shared UI components
│   │   ├── header.go              # Header component
│   │   ├── statusbar.go           # Status bar component
│   │   ├── table.go               # Table component
│   │   ├── spinner.go             # Loading spinner
│   │   └── *_test.go
│   │
│   ├── config/                    # Configuration management
│   │   ├── config.go              # Config struct & loading
│   │   ├── env.go                 # Environment variable handling
│   │   └── *_test.go
│   │
│   ├── api/                       # Shared API client
│   │   ├── client.go              # HTTP client
│   │   ├── types.go               # Common API types
│   │   └── *_test.go
│   │
│   └── tools/                     # Tool implementations
│       ├── mediarequests/
│       │   ├── model.go
│       │   ├── update.go
│       │   ├── view.go
│       │   ├── api.go
│       │   └── *_test.go
│       ├── streams/
│       ├── calendar/
│       ├── queue/
│       └── arrfeed/
│
├── pkg/
│   └── colors/
│       ├── colors.go
│       └── *_test.go
│
└── media-requests.go, media-streams.go, media-calendar.go  # CLI compatibility
```

## Agent Workflow

### Phase 1: Foundation (Agent: Shared Components Lead)

**Worktree Location:** `.worktrees/phase1-foundation/`  
**Branch:** `tui/phase1-foundation`  
**Expected Timeline:** Weeks 1-2

**Tasks:**
1. Create project structure:
   ```bash
   mkdir -p cmd/calms-toolkit internal/{app,components,config,api} pkg/colors
   ```

2. Extract and port shared components:
   - **HTTP Client** (`internal/api/client.go`)
     - Extract from existing media-requests.go:299-322
     - Support all three services: Overseerr, Plex, Jellyfin
     - Unified timeout and retry logic

   - **Color System** (`pkg/colors/colors.go`)
     - Extract ANSI constants from all tools
     - Implement NO_COLOR support
     - Export ColorScheme struct

   - **Configuration** (`internal/config/config.go`)
     - Unified Config struct
     - Load from: CLI flags → env vars → config file → .env → defaults
     - Support all tool configurations

3. Create Bubble Tea foundation:
   - **Main entry point** (`cmd/calms-toolkit/main.go`)
   - **Application model** (`internal/app/model.go`)
   - **Basic tab navigation** system
   - **Window size handling**

4. Write comprehensive unit tests for all components

5. **Commit & PR Strategy:**
   - Create multiple focused commits:
     ```
     feat: create project structure for TUI application
     feat: extract and port HTTP client from existing tools
     feat: extract and port color system to pkg/colors
     feat: implement unified configuration system
     feat: bootstrap Bubble Tea application with tab navigation
     test: add comprehensive unit tests for foundation
     ```
   - Open PR to `tui/integration` branch

**Merge Strategy:** After PR approval, merge to `tui/integration`  
**Blocking:** None - this is the foundation for all other phases

---

### Phase 2: Tool Integration (Individual Agent per Tool)

**Timeline:** Weeks 3-6

#### Phase 2a: Media Requests

**Worktree Location:** `.worktrees/phase2-mediarequests/`  
**Branch:** `tui/phase2-mediarequests`  
**Agent:** Media Requests Developer

**Tasks:**
1. Create tool structure:
   ```
   internal/tools/mediarequests/
   ├── model.go        # Step: search, select, confirm
   ├── update.go       # Message handling
   ├── view.go         # UI rendering
   ├── api.go          # API calls
   └── *_test.go
   ```

2. Port existing functionality:
   - Extract search/request logic from media-requests.go
   - Adapt state machine: search → select → confirm → submit
   - Implement text input component
   - Handle API responses

3. Tests:
   - Unit tests for state transitions
   - Mock API responses
   - Error handling

**Files Modified/Created:**
- `internal/tools/mediarequests/*.go` (all files in this dir)
- `go.mod` (if new dependencies needed)

**Dependencies:**
- Phase 1 foundation must be merged first

**Merge Strategy:** Create PR to `tui/integration`, wait for phase1 merge if not done

---

#### Phase 2b: Streams

**Worktree Location:** `.worktrees/phase2-streams/`  
**Branch:** `tui/phase2-streams`  
**Agent:** Streams Developer

**Tasks:** (Similar to Media Requests but for Streams)
1. Create tool structure in `internal/tools/streams/`
2. Port polling logic to Bubble Tea commands
3. Implement real-time update handling
4. Display active streams in table format

**Files Modified/Created:**
- `internal/tools/streams/*.go` (all files in this dir)

**Dependencies:** Phase 1 foundation

---

#### Phase 2c: Calendar

**Worktree Location:** `.worktrees/phase2-calendar/`  
**Branch:** `tui/phase2-calendar`  
**Agent:** Calendar Developer

**Tasks:** (Similar but for Calendar)
1. Create tool structure in `internal/tools/calendar/`
2. Implement calendar view
3. Port filtering and navigation logic
4. Handle date-based queries

**Files Modified/Created:**
- `internal/tools/calendar/*.go` (all files in this dir)

**Dependencies:** Phase 1 foundation

---

#### Phase 2d: Queue Remediation

**Worktree Location:** `.worktrees/phase2-queue/`  
**Branch:** `tui/phase2-queue`  
**Agent:** Queue Remediation Developer

**Tasks:** (Similar but for Queue)
1. Create tool structure in `internal/tools/queue/`
2. Port queue analysis and remediation logic
3. Implement action selection UI
4. Handle bulk operations

**Files Modified/Created:**
- `internal/tools/queue/*.go` (all files in this dir)

**Dependencies:** Phase 1 foundation

---

### Phase 3: Polish & Integration

#### Phase 3a: ARR Feed Integration

**Worktree Location:** `.worktrees/phase3-arrfeed/`  
**Branch:** `tui/phase3-arrfeed`  
**Agent:** ARR Feed Developer

**Tasks:**
1. Create tool structure in `internal/tools/arrfeed/`
2. Port arr-feed functionality
3. Implement real-time activity display
4. Format event display

**Files Modified/Created:**
- `internal/tools/arrfeed/*.go` (all files in this dir)

**Dependencies:** Phase 1 + all phase 2 tools merged

---

#### Phase 3b: Polish & Testing

**Worktree Location:** `.worktrees/phase3-polish/`  
**Branch:** `tui/phase3-polish`  
**Agent:** Polish & Testing Lead

**Tasks:**
1. Integration testing (e2e workflows)
2. Performance optimization
3. UX polish and refinement
4. Update Makefile build targets
5. Comprehensive documentation

**Files Modified/Created:**
- `*_test.go` files (comprehensive test suite)
- `Makefile` (update build targets)
- `docs/TUI_IMPLEMENTATION_GUIDE.md` (new)
- `docs/KEYBOARD_SHORTCUTS.md` (new)
- Minor fixes across all tools

**Dependencies:** All previous phases merged

---

## Coordination Protocols

### Daily Sync Requirements

- **Morning:** Check this document for scope changes
- **Before Push:** Verify your worktree scope hasn't changed
- **Conflict Detection:** If you need files outside your scope, communicate in `#development` channel

### File Ownership Matrix

| Directory | Owner/Phase | Status | Notes |
|-----------|------------|--------|-------|
| `cmd/calms-toolkit/` | Phase 1 | Foundation | Entry point only |
| `internal/app/` | Phase 1 | Foundation | Central model - coords with all |
| `internal/components/` | Phase 1 | Foundation | Shared UI components |
| `internal/config/` | Phase 1 | Foundation | Config loading |
| `internal/api/` | Phase 1 | Foundation | HTTP client |
| `pkg/colors/` | Phase 1 | Foundation | Color system |
| `internal/tools/mediarequests/` | Phase 2a | Media Requests | Exclusive ownership |
| `internal/tools/streams/` | Phase 2b | Streams | Exclusive ownership |
| `internal/tools/calendar/` | Phase 2c | Calendar | Exclusive ownership |
| `internal/tools/queue/` | Phase 2d | Queue | Exclusive ownership |
| `internal/tools/arrfeed/` | Phase 3a | ARR Feed | Exclusive ownership |
| `docs/` | Phase 3b | Polish | Testing/docs |
| `Makefile` | Phase 1 + 3b | Shared | Phase 1 initial, 3b updates |
| `go.mod`, `go.sum` | All | Shared | Coordinate dependency additions |

### Merge Flow

```
Individual Worktree Branch
    ↓
    └─ [Agent: Work locally, write tests, format with gofmt]
    ↓
Create PR to tui/integration
    ↓
    └─ [Code Review: Verify scope, tests, style]
    ↓
Merge to tui/integration
    ↓
    └─ [Integration Branch: Coordinate with other merged work]
    ↓
Final: Merge tui/integration → main
    ↓
    └─ [Release: Tag and deploy]
```

### Conflict Prevention Strategy

**Workspace Isolation:**
- Phase 1: ONLY touches `cmd/`, `internal/{app,components,config,api}`, `pkg/colors/`, `go.mod`
- Phase 2 tools: ONLY touch `internal/tools/{specific-tool}/`
- Phase 3: Touches everything for final integration

**If You Need Files Outside Your Scope:**
1. ⛔ **DO NOT** modify them directly
2. ✅ **DO** create an issue/discussion linking your worktree
3. ✅ **DO** communicate with the owning phase's agent
4. ✅ **DO** wait for approval before modifying shared files

### Dependency Coordination

**Adding External Dependencies:**
1. First, check if it's already in `go.mod`
2. If adding new dependency, announce in coordination channel
3. All agents: Run `go mod tidy` before pushing
4. Update `go.mod` and `go.sum` together (don't commit one without other)

**Go Module Files** (`go.mod`, `go.sum`):
- Shared resource - coordinate before adding dependencies
- Always run `go mod tidy` before committing
- Phase 1 agent resolves conflicts if they occur

---

## Quick Start for Agents

### Starting Work on Your Worktree

```bash
# Clone the repo (if you haven't)
git clone https://github.com/calmcacil/CalmsToolkit.git
cd CalmsToolkit

# List available worktrees
git worktree list

# Switch to your worktree
cd .worktrees/phase2-mediarequests/

# Verify you're on the right branch
git branch -v

# Pull latest from integration (check for new foundation work)
git pull origin tui/phase2-mediarequests
git merge tui/integration  # Bring in recent foundation updates

# Create a new branch for your specific feature
git checkout -b feature/mediarequests-search-ui

# ... do your work ...

# Commit with clear, imperative messages
git add internal/tools/mediarequests/
git commit -m "feat: implement media search UI component with Bubble Tea"

# Push to your branch
git push origin feature/mediarequests-search-ui

# Open PR to tui/integration (not main!)
```

### Pulling Latest Foundation Updates

When Phase 1 work is merged to `tui/integration`, you should pull those updates:

```bash
cd .worktrees/phase2-mediarequests/

# Fetch latest
git fetch origin

# Merge foundation updates
git merge origin/tui/integration

# Resolve any conflicts (should be minimal if scopes are respected)
# Then continue working
```

### Building and Testing

```bash
cd /Users/samanthamyers/Development/CalmsToolkit

# Build TUI application
make build-tui  # (or: go build -o bin/calms-toolkit ./cmd/calms-toolkit)

# Run tests from your worktree
cd .worktrees/phase2-mediarequests/
go test ./...

# Test integration
cd /Users/samanthamyers/Development/CalmsToolkit
go test ./...
```

---

## Worktree Maintenance

### Checking Worktree Status

```bash
cd /Users/samanthamyers/Development/CalmsToolkit

# List all worktrees
git worktree list

# Show worktree details
git worktree list -v

# Check specific worktree branch
cd .worktrees/phase2-mediarequests/
git status
git log --oneline -5
```

### Cleaning Up Worktrees

When a phase is complete and merged:

```bash
# Verify all work is pushed and merged
cd /Users/samanthamyers/Development/CalmsToolkit
git worktree remove .worktrees/phase1-foundation

# Prune any dead worktree references
git worktree prune
```

### Syncing Worktrees

If your worktree gets out of sync with the integration branch:

```bash
cd /Users/samanthamyers/Development/CalmsToolkit/.worktrees/phase2-mediarequests/

# Fetch latest changes
git fetch origin

# Reset to a clean state (be careful - backs up uncommitted changes)
git stash  # Save any uncommitted changes

# Sync with phase branch
git checkout tui/phase2-mediarequests
git pull origin tui/phase2-mediarequests

# Merge latest from integration (foundation updates)
git merge origin/tui/integration --no-edit

# Restore your work if needed
git stash pop
```

---

## Communication Channels

- **Coordination:** Check this file for scope and merge status
- **Conflicts:** Create an issue linking worktrees involved
- **Questions:** Check .opencode/CLAUDE.md for implementation guidance
- **Integration Issues:** Alert the integration worktree maintainer

---

## Phase Status Tracker

| Phase | Branch | Status | Lead | Target Completion |
|-------|--------|--------|------|------------------|
| Phase 1 | `tui/phase1-foundation` | 🟡 Waiting | TBD | Nov 10 |
| Phase 2a | `tui/phase2-mediarequests` | 🟡 Waiting | TBD | Nov 17 |
| Phase 2b | `tui/phase2-streams` | 🟡 Waiting | TBD | Nov 24 |
| Phase 2c | `tui/phase2-calendar` | 🟡 Waiting | TBD | Dec 1 |
| Phase 2d | `tui/phase2-queue` | 🟡 Waiting | TBD | Dec 8 |
| Phase 3a | `tui/phase3-arrfeed` | 🟡 Waiting | TBD | Dec 15 |
| Phase 3b | `tui/phase3-polish` | 🟡 Waiting | TBD | Dec 22 |

---

**Document Version:** 1.0  
**Created:** November 3, 2025  
**Next Review:** After Phase 1 begins
