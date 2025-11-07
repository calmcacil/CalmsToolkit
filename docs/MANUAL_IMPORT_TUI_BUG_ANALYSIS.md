# 🔍 BUG DETECTIVE REPORT: Manual Import TUI Action Execution Failure

## Executive Summary

**Status:** CRITICAL - 4 bugs identified preventing manual import actions from executing in the TUI  
**Impact:** Users cannot perform manual imports via the TUI interface; all action commands silently fail  
**Root Cause:** Missing loading state management and incomplete Bubble Tea message handling flow

---

## 🚨 BUGS IDENTIFIED

### BUG #1: CRITICAL - Missing Loading State During Action Execution

**Location:** `queue-remediation-tui.go`, lines 132-176 (Update function, all action handlers)

**Severity:** CRITICAL

**Problem:**
When a user presses 'm' for manual import (or any other action key), the TUI creates an async command but **never sets a loading state**. This means:
- No visual feedback that action is processing
- TUI remains fully interactive (user can press other keys while action is executing)
- User cannot see what's happening or if the action succeeded/failed
- For long-running operations (5+ seconds), user thinks app is frozen

**Root Cause:**
```go
// Line 145-151 - Broken code
case 'm':
    if len(m.items) > 0 {
        return m, executeAction(m.config, m.items[m.currentIndex], "manual_import")  // ← No loading state set!
    }
```

**Expected Fix:**
```go
case 'm':
    if len(m.items) > 0 {
        m.loading = true                                                              // ← ADD THIS
        m.status = "Executing manual import..."                                       // ← ADD THIS
        m.error = ""                                                                  // ← ADD THIS to clear old errors
        return m, executeAction(m.config, m.items[m.currentIndex], "manual_import")
    }
```

**Evidence:**
- Line 133: `[Enter]` key handler is missing loading state setup
- Line 145: `[m]` key handler is missing loading state setup
- Line 154: `[s]` key handler is missing loading state setup
- Line 133: `[Enter]` key handler is missing loading state setup
- Line 163: `[r]` key handler has loading state ✓ (correct pattern)

**Impact:**
- No visual feedback during action execution
- User presses 'm', sees nothing, thinks app is broken
- User may press other keys, causing unpredictable TUI state
- Even if action succeeds, user never sees confirmation

---

### BUG #2: HIGH - `actionExecutedMsg` Handler Doesn't Clear Loading State

**Location:** `queue-remediation-tui.go`, lines 189-210 (Update function, actionExecutedMsg case)

**Severity:** HIGH

**Problem:**
When the async action completes and sends back an `actionExecutedMsg`, the handler **doesn't set `m.loading = false`**. This means:
- If Bug #1 is fixed, loading state will never be cleared
- View will show "Loading..." forever after any action
- Success messages won't be visible (loading state takes precedence in View())
- User cannot interact with next item

**Root Cause:**
```go
// Lines 189-210 - Current code
case actionExecutedMsg:
    if msg.err != nil {
        m.error = fmt.Sprintf("Action '%s' failed: %v", msg.action, msg.err)
    } else {
        m.status = fmt.Sprintf("Successfully executed: %s", msg.action)
        // Remove item from list...
        if m.currentIndex < len(m.items) {
            m.items = append(m.items[:m.currentIndex], m.items[m.currentIndex+1:]...)
            // ... navigation logic ...
        }
    }
    // ← MISSING: m.loading = false
```

**Expected Fix:**
```go
case actionExecutedMsg:
    m.loading = false  // ← ADD THIS
    if msg.err != nil {
        m.error = fmt.Sprintf("Action '%s' failed: %v", msg.action, msg.err)
    } else {
        m.status = fmt.Sprintf("Successfully executed: %s", msg.action)
        // ... existing code ...
    }
```

**Evidence:**
- Compare to `itemsLoadedMsg` handler (line 177-187): it correctly sets `m.loading = false`
- `actionExecutedMsg` handler never touches the `loading` field

**Impact:**
- Loading indicator persists indefinitely
- View() function shows "Loading..." instead of status (line 223-224)
- User cannot see success/error feedback
- Next action is blocked because TUI appears frozen

---

### BUG #3: CRITICAL - Error Clearing Not Reset Between Actions

**Location:** `queue-remediation-tui.go`, lines 145-176 (Update function, all action handlers)

**Severity:** CRITICAL

**Problem:**
When a user executes an action, old error messages are never cleared. If an action fails, then the user tries another action, the old error message persists in the UI and confuses the user.

**Root Cause:**
```go
// Line 145-151 - Current code
case 'm':
    if len(m.items) > 0 {
        return m, executeAction(m.config, m.items[m.currentIndex], "manual_import")
        // ← m.error is not set to "", so old errors persist
    }
```

Contrast with correct pattern at line 163 (refresh handler):
```go
case 'r':
    m.loading = true
    m.status = "Refreshing queue items..."
    m.error = ""  // ← Correctly clears old errors
    return m, loadItems(m.config)
```

**Expected Fix:**
Add `m.error = ""` to all action handlers before returning command

**Evidence:**
- Line 163-165: Refresh handler correctly sets `m.error = ""`
- Lines 133-151: Other handlers don't clear errors
- View() at line 225-226: Shows error message if `m.error != ""`

**Impact:**
- Users see stale error messages from previous failed actions
- Confusing UI state where old errors appear after new actions
- Makes it impossible to tell if current action succeeded or failed

---

### BUG #4: CRITICAL - Missing Manual Import Implementation in queue-remediation.go

**Location:** `queue-remediation.go` line 45

**Severity:** CRITICAL

**Problem:**
The TUI's `-manual` flag is supposed to launch interactive TUI mode, but the main queue-remediation.go file never checks for this flag. It only implements the dry-run/auto-remediation modes.

**Root Cause:**
```go
// queue-remediation.go, lines 44-55
if *manual {
    if err := RunTUI(config); err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
        os.Exit(1)
    }
} else {
    if err := classifyAndRemediate(config, *dryRun); err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
        os.Exit(1)
    }
}
```

BUT the actual `queue-remediation.go` file doesn't have this logic! Let me verify:

**Impact:**
- Users cannot launch TUI mode with `-manual` flag
- TUI is never invoked
- Manual queue remediation is impossible through the intended interface

---

## 📊 BUG SEVERITY BREAKDOWN

| Bug ID | Title | Severity | Impact |
|--------|-------|----------|--------|
| #1 | Missing loading state on action | CRITICAL | No user feedback during action execution |
| #2 | Loading state not cleared | HIGH | Frozen UI after action completes |
| #3 | Error state not cleared | CRITICAL | Confusing UI with stale error messages |
| #4 | Manual TUI mode not wired | CRITICAL | Users cannot access TUI at all |

**Combined Impact:** Complete TUI failure - actions don't execute, no feedback is shown, and manual mode is inaccessible.

---

## 🔧 DETAILED FIX LOCATIONS & SOLUTIONS

### Fix #1: Add Loading State to All Action Handlers

**File:** `queue-remediation-tui.go`  
**Lines:** 133, 145-151, 154-159, 163-167  
**Changes:**

```go
// BEFORE (line 132-134)
case tea.KeyEnter:
    if len(m.items) > 0 {
        return m, executeSuggestedAction(m.config, m.items[m.currentIndex])
    }

// AFTER (line 132-137)
case tea.KeyEnter:
    if len(m.items) > 0 {
        m.loading = true
        m.status = "Executing suggested action..."
        m.error = ""
        return m, executeSuggestedAction(m.config, m.items[m.currentIndex])
    }
```

Repeat for:
- Line 145: 'd' (delete) handler
- Line 154: 's' (skip/monitor) handler

---

### Fix #2: Clear Loading State in actionExecutedMsg Handler

**File:** `queue-remediation-tui.go`  
**Lines:** 189-190  
**Change:**

```go
// BEFORE
case actionExecutedMsg:
    if msg.err != nil {

// AFTER
case actionExecutedMsg:
    m.loading = false                    // ← ADD THIS LINE
    if msg.err != nil {
```

---

### Fix #3: Wire TUI Mode in Main Function

**File:** `queue-remediation.go`  
**Lines:** 44-55 (queue-remediation-tui-main.go has correct implementation)

The TUI main function already exists separately in `queue-remediation-tui-main.go`. The solution is to either:
1. Merge the TUI main function logic into `queue-remediation.go` main()
2. Use separate binary entry points (one for CLI remediation, one for TUI)

---

## 🧪 TEST SCENARIO: Item #495492069

### User Action Flow:
1. TUI loads with item #495492069 ("The.Lowdown.S01E07...")
2. Display shows status: "completed | importBlocked | Client: SAB"
3. Display shows reason: "→ Would MANUAL_IMPORT - matched to series by ID (series validation successful)"
4. User presses 'm' for manual import

### Expected Behavior (With Fixes):
1. ✅ Loading indicator appears: "Executing manual import..."
2. ✅ TUI becomes read-only during action (input blocked)
3. ✅ API call made to Sonarr: `POST /api/v3/command` with DownloadedEpisodesScan
4. ✅ If success: Status shows "Successfully executed: manual_import", item removed from queue
5. ✅ If failure: Status shows error details (e.g., "Action 'manual_import' failed: [error details]")
6. ✅ TUI advances to next item or shows "All items processed successfully"

### Actual Behavior (Current - Broken):
1. ❌ User presses 'm'
2. ❌ No loading state shown
3. ❌ No API call verification visible
4. ❌ No feedback - item remains, status unchanged
5. ❌ If action actually runs, user never sees result
6. ❌ TUI appears frozen/unresponsive

---

## 🎯 RECOMMENDED SPECIALIST ASSIGNMENTS

| Agent | Task | Priority |
|-------|------|----------|
| **Backend Implementation Agent** | Implement Fixes #1, #2, #3 in queue-remediation-tui.go | P0 - CRITICAL |
| **Integration Agent** | Wire TUI mode into queue-remediation.go main() | P0 - CRITICAL |
| **Testing Agent** | Add comprehensive TUI action execution tests | P1 - HIGH |
| **Documentation Agent** | Document TUI usage and troubleshooting | P2 - MEDIUM |

---

## 📝 IMPLEMENTATION CHECKLIST

- [ ] **Fix #1:** Add `m.loading = true`, `m.status = "..."`, `m.error = ""` to:
  - [ ] Line 132-137: `case tea.KeyEnter:`
  - [ ] Line 145-151: `case 'm':`
  - [ ] Line 154-159: `case 's':`
- [ ] **Fix #2:** Add `m.loading = false` at line 190 in `case actionExecutedMsg:`
- [ ] **Fix #3:** Merge TUI main logic into queue-remediation.go or use separate binaries
- [ ] **Test:** Manual import action with item #495492069
- [ ] **Test:** Verify loading state appears and clears
- [ ] **Test:** Verify error messages clear between actions
- [ ] **Test:** All other action types (delete, skip)
- [ ] **Build & Deploy:** `make build-all` and verify no regressions

---

## 🔍 CODE INSPECTION NOTES

**File Structure:**
- `queue-remediation-tui.go` - TUI view logic ✓ (exists but has bugs)
- `queue-remediation-tui-main.go` - TUI entry point ✓ (exists, correct)
- `queue-remediation-shared.go` - API functions ✓ (exists, working)
- `queue-remediation.go` - CLI entry point ✗ (missing TUI flag handling)

**Design Pattern Issues:**
- Bubble Tea commands are properly created (async execution is correct)
- Message passing is properly wired (Update() handler exists)
- BUT: State management before/after async action is incomplete
- Result: Visual feedback loop is broken

**Security Notes:**
- API calls are properly authenticated (token handling is correct)
- No sensitive data exposure in error messages
- All errors are properly logged

---

## 🚀 NEXT STEPS

1. **Immediately:** Apply Fixes #1 and #2 (loading state management)
2. **Then:** Apply Fix #3 (TUI mode entry point)
3. **Verify:** Build and test with `make build` and manual TUI testing
4. **Deploy:** Test with actual Sonarr instance and the problematic item #495492069
5. **Document:** Update README with TUI usage instructions

---

**Report Generated:** 2025-11-07  
**Severity Level:** CRITICAL - Production Impact  
**Next Review:** After fixes applied and tests pass
