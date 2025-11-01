# Queue Remediation Documentation Index

**Last Updated:** November 1, 2025  
**Status:** Complete Research + Implementation Guidance

This index helps navigate the queue remediation research and implementation guidance.

## 📋 Quick Navigation

### For Implementation (Backend)
→ **Start here:** `STATUS_MESSAGE_ROUTING.md`
- Read: Executive Summary + Core Routing Logic
- Reference: Implementation Guidance + Go Pattern
- Use: Pseudocode and decision trees as implementation guide
- Follow: Code Review Checklist before committing

### For Testing (QA)
→ **Start here:** `STATUS_MESSAGE_ROUTING.md`
- Read: Test Cases + Blocklist Decision Matrix
- Reference: All 7 Decision Tree Scenarios
- Use: 14-row lookup matrix to verify test coverage
- Verify: Code Review Checklist passes

### For API Integration
→ **Start here:** `SONARR_RADARR_QUEUE_API.md`
- Reference: Queue item structure and status values
- Learn: How statusMessages array is populated
- Understand: trackedDownloadState and trackedDownloadStatus meanings
- Quick lookup: `SONARR_RADARR_QUEUE_API_QUICK_REFERENCE.md`

### For Architecture Review
→ **Start here:** `QUEUE_REMEDIATION_RESEARCH.md`
- Overview: Research methodology and findings
- Context: Live testing data from production
- Strategy: Remediation approach and priorities
- References: All supporting documentation

---

## 📚 Document Guide

### Primary: STATUS_MESSAGE_ROUTING.md (642 lines)
**Purpose:** Detailed routing logic for handling multiple status messages  
**Contains:**
- 6 status message types with detection patterns
- 7-tier priority system
- 7 decision tree scenarios with real examples
- 14-row blocklist decision matrix
- Pseudocode + Go implementation patterns
- 4 new test case examples
- 5 FAQ pairs with answers
- ASCII flowchart
- 12-point code review checklist

**Use When:**
- Implementing new parseStatusMessages() logic
- Deciding how to handle multi-message scenarios
- Writing or reviewing tests
- Determining blocklist decisions

**Key Sections:**
1. Status Message Types Reference (what each message means)
2. Core Routing Logic (7-tier priority system)
3. Decision Trees (7 detailed scenarios)
4. Blocklist Decision Matrix (14 combinations)
5. Implementation Guidance (pseudocode + Go)
6. Migration Path (what changes are needed)
7. Code Review Checklist (verification)

---

### API Reference: SONARR_RADARR_QUEUE_API.md (790 lines)
**Purpose:** Complete API documentation for Sonarr/Radarr queue endpoints  
**Contains:**
- Authentication and base URL configuration
- All queue endpoints (GET, DELETE, POST, etc.)
- Complete JSON response structure
- Queue item fields reference (35+ fields documented)
- Queue status values and meanings
- Real-world status message examples
- Pagination and filtering
- Go integration examples

**Use When:**
- Implementing API calls
- Understanding queue item structure
- Debugging API responses
- Handling redirects and errors

**Key Sections:**
1. Authentication (X-Api-Key headers)
2. Queue Endpoints (GET/DELETE/POST)
3. JSON Response Structure (complete item structure)
4. Queue Status Values (status enums and transitions)
5. Real-World Status Messages (actual production examples)
6. Go Code Integration (working examples)

---

### Quick Reference: SONARR_RADARR_QUEUE_API_QUICK_REFERENCE.md (308 lines)
**Purpose:** Fast lookup guide for common operations  
**Contains:**
- Essential facts in table format
- Critical status fields
- Key response fields
- Common operations with curl examples
- Go integration quick start
- Status combinations table
- Real-world error examples

**Use When:**
- Need quick API reference
- Writing curl commands
- Looking up status values
- Debugging quickly

---

### Strategy Overview: QUEUE_REMEDIATION_RESEARCH.md (330 lines)
**Purpose:** Research summary and remediation strategy  
**Contains:**
- Live queue analysis from 5 instances
- Status patterns identified
- API documentation validation
- Remediation strategy overview
- Test implementation guidance
- Mock data for testing
- Implementation checklist

**Use When:**
- Understanding the bigger picture
- Learning what was discovered in testing
- Finding test data examples
- Reviewing research methodology

---

## 🎯 Implementation Workflow

### Phase 1: Planning (Read These)
1. ✅ QUEUE_REMEDIATION_RESEARCH.md - Understand context
2. ✅ STATUS_MESSAGE_ROUTING.md (Executive Summary) - Understand problem
3. ✅ STATUS_MESSAGE_ROUTING.md (Core Routing Logic) - Understand solution

### Phase 2: Implementation (Reference These)
1. STATUS_MESSAGE_ROUTING.md (Implementation Guidance)
   - Pseudocode for new parseStatusMessages()
   - Updated mapStatusToAction() pseudocode
   - Go implementation pattern
2. SONARR_RADARR_QUEUE_API.md (if needed)
   - For understanding API details
3. queue-remediation.go
   - Current implementation to modify

### Phase 3: Testing (Verify Against These)
1. STATUS_MESSAGE_ROUTING.md (Test Cases section)
   - 4 new test scenarios provided
2. STATUS_MESSAGE_ROUTING.md (Blocklist Decision Matrix)
   - 14 combinations to test against
3. STATUS_MESSAGE_ROUTING.md (Decision Trees)
   - 7 scenarios to verify

### Phase 4: Code Review (Use This)
1. STATUS_MESSAGE_ROUTING.md (Code Review Checklist)
   - 12-point verification checklist

---

## 🔑 Key Insights

### The Problem
**Current:** First-match logic misses full context when multiple messages exist

```
Item with messages: ["Sample", "Not a Custom Format upgrade"]
Current logic: Matches "Sample" first → returns sample_file
Result: Deletes WITHOUT blocklist (WRONG!)
Should: Recognize CF issue is primary → DELETE WITH blocklist
```

### The Solution
**New:** Scan ALL messages and apply priority-based routing

**Priorities (Highest → Lowest):**
1. Quality/Custom Format issues (always blocklist)
2. Quality/CF + Sample (CF wins)
3. Sample alone (no blocklist)
4. No Files Found (no blocklist)
5. ID Mismatch (manual import)
6. importBlocked alone (manual import)
7. Unknown (monitor)

### Function Changes
```
OLD: parseStatusMessages() -> string
NEW: parseStatusMessages() -> (string, bool)
     Returns: (action, shouldBlocklist)
```

---

## ✅ Verification Checklist

### Before Implementation
- [ ] Read STATUS_MESSAGE_ROUTING.md completely
- [ ] Understand 7-tier priority system
- [ ] Review all 14 blocklist decision matrix rows
- [ ] Review all 7 decision tree scenarios
- [ ] Understand pseudocode logic

### During Implementation
- [ ] Follow Go implementation pattern from document
- [ ] Scan ALL messages (not first-match)
- [ ] Update function signature to return (string, bool)
- [ ] Implement priority logic correctly
- [ ] Handle importBlocked state explicitly

### During Testing
- [ ] All existing single-message tests pass
- [ ] New multi-message test cases added
- [ ] All 14 matrix combinations tested
- [ ] Edge cases handled correctly
- [ ] importBlocked scenarios verified

### Before Code Review
- [ ] Check against 12-point checklist
- [ ] All scenarios from decision trees work
- [ ] Blocklist decisions match matrix
- [ ] Edge cases documented
- [ ] Tests are comprehensive

---

## 📊 Status Message Types

| Type | Pattern | Priority | Action | Blocklist |
|------|---------|----------|--------|-----------|
| Quality Revision | "quality revision upgrade" | TIER 1 | DELETE | TRUE |
| Custom Format | "custom format upgrade" | TIER 1 | DELETE | TRUE |
| Sample | "sample" | TIER 3 | DELETE | FALSE* |
| No Files Found | "no files found" | TIER 4 | DELETE | FALSE |
| ID Mismatch | "matched to series by id" | TIER 5 | MANUAL | N/A |
| importBlocked | trackedDownloadState | TIER 6 | CONTEXT | VARIES |

*Sample has FALSE blocklist ONLY if it's the sole issue. If combined with Quality/CF, follows CF decision (TRUE).

---

## 🚀 For Different Agent Roles

### backend-architect-2025
**Primary Document:** STATUS_MESSAGE_ROUTING.md
**Sections to Focus:**
- Implementation Guidance (pseudocode + Go pattern)
- Pseudocode for parseStatusMessages()
- Go Implementation Pattern
- Migration Path

**Deliverable:** Updated queue-remediation.go functions

---

### QA / Testing Agent
**Primary Documents:** 
- STATUS_MESSAGE_ROUTING.md (Test Cases + Matrix)
- queue-remediation_test.go (existing tests)

**Sections to Focus:**
- Test Cases for New Logic (4 scenarios)
- Blocklist Decision Matrix (verification)
- Decision Trees (coverage validation)

**Deliverable:** Updated test_remediation.go with new scenarios

---

### Code Review / Quality Agent
**Primary Document:** STATUS_MESSAGE_ROUTING.md
**Sections to Focus:**
- Code Review Checklist (12 points)
- All Decision Trees (coverage)
- Blocklist Matrix (correctness)

**Deliverable:** Code review approval/feedback

---

### Documentation / Integration Agent
**Primary Documents:**
- STATUS_MESSAGE_ROUTING.md (complete reference)
- SONARR_RADARR_QUEUE_API.md (API context)
- README.md updates (user-facing docs)

**Sections to Focus:**
- Overview and Executive Summary
- Visual Quick Reference
- FAQ & Edge Cases

**Deliverable:** Updated README and user documentation

---

## 📞 Document Cross-References

### STATUS_MESSAGE_ROUTING.md References:
- SONARR_RADARR_QUEUE_API.md (for API context)
- SONARR_RADARR_QUEUE_API_QUICK_REFERENCE.md (for quick lookup)
- queue-remediation.go (lines 79-107, parseStatusMessages)
- queue-remediation.go (lines 52-77, mapStatusToAction)
- queue-remediation_test.go (test coverage)

### SONARR_RADARR_QUEUE_API.md References:
- Related in ARR Feed Spec: docs/ARR_FEED_SPEC.md
- Related Overseerr: docs/OVERSEERR_API_RESEARCH.md
- Project guidelines: CLAUDE.md, AGENTS.md

### QUEUE_REMEDIATION_RESEARCH.md References:
- Validation report: docs/API_DOCUMENTATION_VALIDATION_REPORT.md
- Test summary: docs/TEST_SUMMARY.md
- Examples: docs/EXAMPLES_MEDIA_REQUESTS.md

---

## 📈 Project Structure

```
CalmsToolkit/
├── queue-remediation.go           (Main implementation - 598 lines)
├── queue-remediation_test.go      (Tests - 1278 lines)
├── docs/
│   ├── STATUS_MESSAGE_ROUTING.md  ← NEW (642 lines, 25KB)
│   ├── SONARR_RADARR_QUEUE_API.md (790 lines, API reference)
│   ├── SONARR_RADARR_QUEUE_API_QUICK_REFERENCE.md (308 lines)
│   ├── QUEUE_REMEDIATION_RESEARCH.md (330 lines, strategy)
│   ├── API_DOCUMENTATION_VALIDATION_REPORT.md
│   ├── ARR_FEED_SPEC.md
│   └── [other docs]
├── QUEUE_REMEDIATION_INDEX.md     ← NEW (this file)
├── README.md
├── CLAUDE.md
└── AGENTS.md
```

---

## ✨ Next Steps

1. **Immediately:** Read STATUS_MESSAGE_ROUTING.md (Executive Summary)
2. **Planning:** Read Core Routing Logic + Decision Trees
3. **Implementation:** Follow Implementation Guidance + Go Pattern
4. **Testing:** Verify against Test Cases + Blocklist Matrix
5. **Review:** Check against 12-point Code Review Checklist

---

**Questions?** Refer to FAQ & Edge Cases section in STATUS_MESSAGE_ROUTING.md

**Ready to start?** Go to STATUS_MESSAGE_ROUTING.md
