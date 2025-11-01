# Manual Import Implementation Guide for queue-remediation.go

**Prepared:** November 1, 2025  
**Target:** CalmsToolkit queue-remediation tool enhancement  
**Status:** Research complete, recommendations ready for implementation

---

## Executive Summary

This document provides specific recommendations for enhancing the `queue-remediation.go` tool to properly handle manual imports detected in Sonarr and Radarr queues. The current implementation uses the Command API (fire-and-forget background scanning), which has limitations for items marked as "import blocked" or "matched by ID". A REST API approach offers better control and targeted remediation.

---

## Current State Analysis

### What Works Well
- ✅ Queue item detection and classification
- ✅ Identification of import-blocked and matched-by-ID items
- ✅ Output path extraction from queue items
- ✅ Basic command API triggering

### Limitations
- ❌ Cannot target specific series/movie IDs
- ❌ Items may remain import-blocked after command execution
- ❌ No validation of file matching before import
- ❌ Limited feedback on import success/failure
- ❌ Bulk scanning may process unrelated files

---

## Recommended Enhancement Path

### Phase 1: Immediate (Low Risk, High Value)

**Objective:** Enhance current implementation with targeted REST API for import-blocked items

**Changes to `queue-remediation.go`:**

1. **Add new function: `scanForImportFiles()`**
   ```go
   func scanForImportFiles(config Config, url string, token string, 
       downloadPath string, instanceType string) ([]ManualImportItem, error)
   ```
   - Calls `GET /api/v3/manualimport?folder={path}&filterExistingFiles=true`
   - Returns array of files with matching information
   - Filters out items with rejections

2. **Add new function: `executeManualImportWithIDs()`**
   ```go
   func executeManualImportWithIDs(config Config, url string, token string, 
       items []ManualImportRequest, instanceType string) error
   ```
   - Calls `POST /api/v3/manualimport` with explicit series/movie IDs
   - Handles Sonarr vs Radarr payload differences
   - Returns import results/rejections

3. **Enhance `triggerManualImport()` function**
   - First attempt: Scan for files with REST API
   - If matches found: Use REST API import with IDs
   - Fallback: Use Command API if REST API unavailable or files have rejections

4. **Add data structures**
   ```go
   type ManualImportItem struct {
       Path        string
       Series      *SeriesInfo // or Movie for Radarr
       SeasonNumber *int
       Episodes    []EpisodeInfo
       Quality     QualityInfo
       Languages   []LanguageInfo
       Rejections  []RejectionInfo
   }
   
   type ManualImportRequest struct {
       Path        string
       SeriesID    int // or MovieID for Radarr
       SeasonNumber int
       EpisodeIDs  []int
       Quality     QualityInfo
       Languages   []LanguageInfo
       DownloadID  string
   }
   ```

**Benefits:**
- Better remediation of import-blocked items
- Explicit ID matching reduces failures
- Maintains backward compatibility with Command API
- Can be implemented incrementally

**Code Complexity:** Medium (≈200-300 lines of new code)

---

### Phase 2: Enhanced (Medium Risk, Medium Value)

**Objective:** Add library search for unmatched files

**Additional Changes:**

1. **Add function: `findSeriesByName()` (Sonarr)**
   - Calls `GET /api/v3/series/lookup?query={name}`
   - Returns best match or list of matches

2. **Add function: `findMovieByName()` (Radarr)**
   - Calls `GET /api/v3/movie/lookup?query={name}`
   - Returns best match or list of matches

3. **Add filename parsing**
   - Extract series/movie name from file path
   - Use library search as fallback for unmatched files

4. **Implement smart matching**
   - Match against queue item title
   - Use fuzzy matching if exact match fails
   - Cache results to avoid repeated lookups

**Benefits:**
- Can remediate files not matched by auto-detection
- Reduces manual intervention needed
- Better handling of rename-heavy releases

**Code Complexity:** Medium-High (≈400-500 additional lines)

---

### Phase 3: Advanced (High Risk, High Value)

**Objective:** Full REST API integration with error recovery

**Additional Changes:**

1. **Implement retry logic**
   - Detect common import failures
   - Attempt targeted fixes before manual intervention

2. **Add quality/custom format awareness**
   - Cache profile settings
   - Make informed decisions about import suitability

3. **Implement logging/metrics**
   - Track import success rates
   - Log detailed rejection reasons
   - Enable monitoring/alerting

4. **Add dry-run mode for REST API**
   - Show what would be imported
   - Display potential issues before execution

**Benefits:**
- Near-automatic resolution of most import issues
- Detailed diagnostics for operators
- Data-driven optimization

**Code Complexity:** High (≈800+ additional lines)

---

## Documentation Updates Required

### 1. Update `/docs/SONARR_RADARR_QUEUE_API.md`

**Add new section after "Queue Endpoints":**

```markdown
## Manual Import Endpoints

### Overview

Sonarr and Radarr provide REST API endpoints for scanning download folders 
and executing targeted file imports with explicit series/movie ID matching.

### GET /api/v3/manualimport - Scan for Importable Files

[Include content from MANUAL_IMPORT_API_RESEARCH.md section 1.1]

### POST /api/v3/manualimport - Execute Import with IDs

[Include content from MANUAL_IMPORT_API_RESEARCH.md section 1.2]

### When to Use Manual Import API

[Include decision matrix from MANUAL_IMPORT_API_QUICK_REFERENCE.md]
```

**Location:** After section "## Queue Endpoints" (around line 78)  
**Word count:** ~3,500 words (includes examples and tables)

---

### 2. Update `/docs/SONARR_RADARR_QUEUE_API_QUICK_REFERENCE.md`

**Add new major section:**

```markdown
## Manual Import Operations

[Include content from MANUAL_IMPORT_API_QUICK_REFERENCE.md sections 
starting with "Two Import Approaches" through "Common Operations"]

### Quick Facts

[Include manual import quick facts table]

### Essential Endpoints

[Include manual import endpoints]
```

**Location:** Add at end of existing content, before "Debugging" section  
**Word count:** ~2,000 words

---

### 3. Create `/docs/QUEUE_REMEDIATION_IMPLEMENTATION_GUIDE.md`

**New document covering:**
- Current queue-remediation.go functionality
- How it integrates with queue API
- Implementation patterns for manual imports
- Testing strategies
- Deployment considerations

---

## Testing Strategy

### Unit Tests
- Test scanning with various folder structures
- Test import request building (Sonarr vs Radarr)
- Test rejection handling
- Test error responses

### Integration Tests
- Test against live Sonarr/Radarr instances
- Test various import-blocked scenarios
- Test with different quality profiles
- Test download ID tracking

### Example Test Cases

```go
func TestScanForImportFiles(t *testing.T) {
    // Should successfully scan folder and return items
    // Should filter items with rejections
    // Should handle empty folders
    // Should handle missing series/movie
}

func TestExecuteManualImportWithIDs(t *testing.T) {
    // Should build correct payload for Sonarr
    // Should build correct payload for Radarr
    // Should handle missing series IDs
    // Should handle invalid episode IDs
    // Should track download IDs
}

func TestEnhancedTriggerManualImport(t *testing.T) {
    // Should try REST API first
    // Should fall back to Command API on error
    // Should handle mixed results
    // Should log reasons for import blocks
}
```

---

## Implementation Considerations

### 1. Backward Compatibility
- Keep Command API as fallback option
- Don't break existing `-dry-run` mode
- Maintain same command-line interface

### 2. Error Handling
- Network failures during REST API calls
- Malformed responses
- Edge cases (file deleted during import, permission errors)
- Series/movie not in library

### 3. Performance
- Implement request batching (bundle multiple files in single POST)
- Cache library lookups to avoid repeated API calls
- Consider rate limiting to avoid overwhelming instances
- Typical scan time: 100-500ms, Import time: 500ms-2s

### 4. Logging
- Log scanned file count
- Log matched series/movies
- Log rejections and reasons
- Log import execution results
- Enable debug mode for detailed diagnostics

### 5. Configuration
- New flag: `--use-manual-import-api` (enable new behavior)
- New flag: `--library-cache-timeout` (cache duration)
- New flag: `--import-batch-size` (files per POST request)

---

## Code Structure Recommendation

```
queue-remediation.go
├── Existing functions
│   ├── mapStatusToAction()
│   ├── parseStatusMessages()
│   ├── fetchQueue()
│   ├── deleteQueueItem()
│   ├── triggerManualImport() [ENHANCED]
│   └── ...
│
├── NEW: Manual Import Types
│   ├── type ManualImportResource struct
│   ├── type ManualImportRequest struct
│   ├── type LibraryItem struct (Series/Movie wrapper)
│   └── type ImportResult struct
│
├── NEW: Manual Import Functions
│   ├── scanForImportFiles()
│   ├── executeManualImportWithIDs()
│   ├── findSeriesByName()        [Phase 2]
│   ├── findMovieByName()         [Phase 2]
│   ├── parseFilename()           [Phase 2]
│   └── buildImportRequest()
│
├── REFACTORED: Helper Functions
│   ├── getTokenForInstance()     [Existing]
│   └── classifyAndRemediate()    [ENHANCED for REST API]
│
└── Main entry point
```

---

## Integration with Existing Queue Detection

The current code detects import-blocked items at line 68-74:

```go
case "matched_by_id":
    return "manual_import", false, true
```

and line 72-74:

```go
if item.TrackedDownloadState == "importBlocked" {
    return "manual_import", false, true
}
```

**Enhancement approach:** When `manualImport == true`, attempt REST API import flow before falling back to Command API.

---

## Security Considerations

- ✅ API keys remain in environment variables (no changes needed)
- ✅ HTTPS already supported by existing code
- ✅ No new auth mechanisms required (uses existing X-Api-Key)
- ✅ File paths are from queue responses (trusted source)

---

## Metrics and Monitoring

Recommended metrics to track:

```go
type ImportMetrics struct {
    TotalScanned        int
    SuccessfulImports   int
    FailedImports       int
    BlockedImports      int
    AverageImportTime   float64
    LibraryLookups      int
    CacheHits           int
}
```

---

## Rollout Strategy

**Recommended approach:**

1. **Week 1:** Phase 1 implementation (REST API scan + import)
   - Deploy to staging first
   - Test with live instances
   - Gather metrics

2. **Week 2:** Phase 1 production deployment
   - Gradual rollout to subset of users
   - Monitor error rates
   - Collect feedback

3. **Week 3-4:** Phase 2 implementation (library search)
   - Add filename parsing and fuzzy matching
   - Update documentation
   - Staging testing

4. **Week 5:** Phase 2 production deployment
   - Full rollout with enhanced matching
   - Monitor success rates

5. **Week 6+:** Phase 3 (advanced features)
   - Retry logic and error recovery
   - Enhanced logging and metrics

---

## Migration Path for Existing Users

For users with current queue-remediation deployment:

1. **No breaking changes** - existing flags continue to work
2. **Opt-in** - new REST API behavior controlled by flag
3. **Safe fallback** - if REST API fails, automatically uses Command API
4. **Transparent upgrade** - no operational changes required

---

## Documentation Changes Summary

| File | Change Type | Section | Approx. Words |
|------|-------------|---------|---------------|
| `SONARR_RADARR_QUEUE_API.md` | Addition | Manual Import Endpoints | 3,500 |
| `SONARR_RADARR_QUEUE_API_QUICK_REFERENCE.md` | Addition | Manual Import Operations | 2,000 |
| `NEW: QUEUE_REMEDIATION_IMPLEMENTATION_GUIDE.md` | New file | Complete guide | 2,500 |
| `README.md` (queue-remediation section) | Update | Usage examples | 500 |

**Total documentation:** ~8,500 new words

---

## Success Criteria

Phase 1 completion will be successful when:
- ✅ REST API endpoints properly documented
- ✅ Code implements scan + targeted import workflow
- ✅ 80%+ of import-blocked items successfully remediated
- ✅ No regression in existing queue remediation functionality
- ✅ Command API fallback works reliably
- ✅ Tests pass (unit + integration)
- ✅ Documentation is complete and clear

---

## Risks and Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|-----------|
| API changes between versions | Low | High | Test against v3/v4/v5, maintain version detection |
| Network/timeout issues | Medium | Medium | Implement retry logic with exponential backoff |
| Incorrect series/movie matching | Medium | Medium | Validate matches before import, dry-run mode |
| File permission errors | Low | High | Handle gracefully, log detailed errors |
| Performance degradation | Low | Medium | Implement batching and caching |
| Breaking existing workflows | Low | High | Keep Command API as fallback, use feature flag |

---

## Next Steps

1. **Review this document** with team/stakeholders
2. **Approve implementation approach** (Phase 1, 2, or 3)
3. **Create git branch** for development
4. **Implement Phase 1** following code structure recommendation
5. **Write integration tests** against live instances
6. **Update documentation** using provided templates
7. **Code review** and refinement
8. **Staging deployment** and testing
9. **Production rollout** with monitoring

---

## References

- Complete Research: `/docs/MANUAL_IMPORT_API_RESEARCH.md`
- Quick Reference: `/docs/MANUAL_IMPORT_API_QUICK_REFERENCE.md`
- Queue API Docs: `/docs/SONARR_RADARR_QUEUE_API.md`
- Current Implementation: `/queue-remediation.go` (lines 259-306)

---

**Prepared by:** Research Librarian  
**Date:** November 1, 2025  
**Status:** Ready for implementation planning  
