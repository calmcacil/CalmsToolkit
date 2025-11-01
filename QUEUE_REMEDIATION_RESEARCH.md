# Queue Remediation Research Summary

**Research Date:** November 1, 2025  
**Status:** Documentation Complete - Ready for Test Development  
**Purpose:** Enable automated remediation of stuck Sonarr/Radarr queue items

---

## Overview

This document summarizes research and documentation work completed for implementing queue remediation functionality in CalmsToolkit. The goal is to automatically detect and fix stuck downloads in Sonarr (TV) and Radarr (Movies) download queues based on status patterns.

---

## Work Completed

### 1. Live Queue Analysis

**Instances Analyzed:**
- Sonarr HD (192.168.88.208:8989) - 13 queue items
- Sonarr 4K (192.168.88.209:8989) - 0 queue items
- Sonarr Anime (192.168.88.207:8989) - 0 queue items
- Radarr HD (192.168.88.205:7878) - 1 queue item
- Radarr UHD (192.168.88.206:7878) - 0 queue items

**Key Discovery:** HTTP 307 redirects due to reverse proxy base URLs (e.g., `/sonarrhd/`)

### 2. Status Patterns Identified

**Live Status Combinations:**
- `completed|importPending|warning` - Download done but waiting to import with issues
- `completed|importBlocked|warning` - Download done but blocked from importing

**Real Error Messages Captured:**
- "Not a Custom Format upgrade for existing episode/movie file(s)..."
- "No files found are eligible for import in {path}"
- "Found matching series via grab history, but release was matched to series by ID"
- "Not a quality revision upgrade for existing episode file(s)"
- "Sample"

### 3. API Documentation Created & Validated

**Primary Documentation:**
- `docs/SONARR_RADARR_QUEUE_API.md` - Comprehensive API reference (650+ lines)
- `docs/SONARR_RADARR_QUEUE_API_QUICK_REFERENCE.md` - Quick lookup guide (210+ lines)

**Validation & Updates:**
- `docs/API_DOCUMENTATION_VALIDATION_REPORT.md` - Accuracy validation report
- All inaccuracies fixed (URL redirects, statusMessages field, importBlocked state)
- Real production error messages documented
- 100% production-ready confidence level

---

## Queue Status Field Reference

### Critical Fields for Remediation

```go
type QueueItem struct {
    ID                      int              `json:"id"`
    Title                   string           `json:"title"`
    Status                  string           `json:"status"`
    TrackedDownloadState    string           `json:"trackedDownloadState"`
    TrackedDownloadStatus   string           `json:"trackedDownloadStatus"`
    ErrorMessage           string           `json:"errorMessage"`
    StatusMessages         []StatusMessage  `json:"statusMessages"`
    DownloadClient         string           `json:"downloadClient"`
    DownloadId             string           `json:"downloadId"`
}

type StatusMessage struct {
    Title    string   `json:"title"`
    Messages []string `json:"messages"`
}
```

### Status Value Enums

**status** (primary queue position):
- `downloading` - Actively downloading
- `paused` - Paused by user/client
- `queued` - Waiting to start
- `completed` - Ready for import
- `failed` - Download failed
- `warning` - Has issues but continuing
- `delay` - Delayed by delay profile

**trackedDownloadState** (detailed state):
- `downloading` - Receiving data
- `importPending` - Waiting to import
- `importBlocked` - Blocked from importing (requires intervention)
- `importing` - Currently importing
- `imported` - Successfully imported
- `failedPending` - Failed, pending cleanup
- `failed` - Failed state
- `ignored` - Ignored/skipped

**trackedDownloadStatus** (health indicator):
- `ok` - No issues
- `warning` - Non-critical issues
- `error` - Critical errors

---

## Remediation Strategy

### Classification Logic

**Import Blocked** (requires manual intervention):
- Pattern: `status=completed + trackedDownloadState=importBlocked + trackedDownloadStatus=warning`
- Action: `POST /api/v3/command` with manual import command

**Import Pending with Issues** (can auto-remediate):
- Pattern: `status=completed + trackedDownloadState=importPending + trackedDownloadStatus=warning`
- Actions based on `statusMessages`:
  - "Not a Custom Format upgrade" → Delete + blocklist (won't improve)
  - "No files found eligible" → Delete (missing files)
  - "Sample" → Delete (sample file)
  - "matched to series by ID" → Manual import

### API Endpoints for Remediation

**Query Queue:**
```bash
curl -s -L -H "X-Api-Key: {token}" \
  "http://host:port/api/v3/queue?pageSize=100"
```
Note: `-L` flag required to follow redirects

**Delete Item:**
```bash
curl -X DELETE -H "X-Api-Key: {token}" \
  "http://host:port/api/v3/queue/{id}?removeFromClient=true&blocklist=true"
```

**Trigger Manual Import:**
```bash
curl -X POST -H "X-Api-Key: {token}" \
  -H "Content-Type: application/json" \
  -d '{"name":"DownloadedEpisodesScan","path":"/path/to/downloads"}' \
  "http://host:port/api/v3/command"
```

---

## Test Implementation Guidance

### Test Scenarios to Implement

1. **API Connection Tests**
   - Test with/without `-L` redirect following
   - Test with valid/invalid API keys
   - Test pagination (pageSize parameter)
   - Test against all instance types (Sonarr HD/4K/Anime, Radarr HD/UHD)

2. **Status Parsing Tests**
   - Parse `completed|importPending|warning` combination
   - Parse `completed|importBlocked|warning` combination
   - Extract and parse `statusMessages[]` structure
   - Handle missing/null fields gracefully

3. **Remediation Logic Tests**
   - Classify "Not a Custom Format upgrade" → delete+blocklist action
   - Classify "No files found" → delete action
   - Classify "Sample" → delete action
   - Classify "matched to series by ID" → manual import action
   - Classify importBlocked → manual import action

4. **API Operation Tests**
   - Mock DELETE /api/v3/queue/{id} with query params
   - Mock POST /api/v3/command for manual import
   - Verify correct headers (X-Api-Key)
   - Test error handling for failed API calls

5. **Integration Tests**
   - Test full workflow: query → classify → remediate
   - Test multi-instance handling (3 Sonarr + 2 Radarr instances)
   - Test with empty queues (should not error)
   - Test with mixed status queues

### Mock Data for Testing

**Sample Queue Response:**
```json
{
  "records": [
    {
      "id": 780524587,
      "title": "It.2017.PROPER.1080p.BluRay.X264-DEFLATE-AsRequested",
      "status": "completed",
      "trackedDownloadState": "importPending",
      "trackedDownloadStatus": "warning",
      "errorMessage": "",
      "statusMessages": [
        {
          "title": "It.2017.PROPER.1080p.BluRay.X264-DEFLATE-AsRequested",
          "messages": [
            "Not a Custom Format upgrade for existing movie file(s). New: [Repack/Proper - Notifiarr] (5) do not improve on Existing: [HD Bluray Tier 02 - Notifiarr] (1750)"
          ]
        }
      ],
      "downloadClient": "qBittorrent",
      "downloadId": "ABC123"
    }
  ],
  "page": 1,
  "pageSize": 100,
  "totalRecords": 1
}
```

### Configuration for Tests

Use `.env` file credentials (already configured):
```
SONARR_URLS=http://192.168.88.208:8989,http://192.168.88.209:8989,http://192.168.88.207:8989
SONARR_TOKENS=f0807d0f4f7346a2b0bc595651926656,16ce5e3497c54085a1f2e23bc938e8e1,66c0ea0064f14ce782e966d89e2f379f
RADARR_URLS=http://192.168.88.205:7878,http://192.168.88.206:7878
RADARR_TOKENS=e12463603bed4b42b92d64536fea07b8,de5e8cd6a6164185a6a29eefc2aac4d7
```

---

## Documentation References

### Primary API Documentation
- **Full Reference:** `docs/SONARR_RADARR_QUEUE_API.md`
  - Complete endpoint documentation
  - Field reference tables
  - Real production examples
  - Go code examples
  
- **Quick Reference:** `docs/SONARR_RADARR_QUEUE_API_QUICK_REFERENCE.md`
  - Fast lookup guide
  - Common operations
  - curl command examples
  - Status value tables

### Validation & Research
- **Validation Report:** `docs/API_DOCUMENTATION_VALIDATION_REPORT.md`
  - Live testing methodology
  - Inaccuracies found and fixed
  - Production readiness assessment
  
- **Related Docs:**
  - `docs/SONARR_RADARR_QUEUE_API_QUICK_REFERENCE.md` - Quick reference guide
  - `docs/ARR_FEED_SPEC.md` - ARR feed specification
  - `CLAUDE.md` - Project AI assistant guidelines

---

## Implementation Checklist

### For Test Developers

- [ ] Read `docs/SONARR_RADARR_QUEUE_API.md` completely
- [ ] Review live testing data in this document
- [ ] Create test file: `queue-remediation_test.go`
- [ ] Implement queue parsing tests (status combinations)
- [ ] Implement statusMessages parsing tests
- [ ] Implement remediation classification logic tests
- [ ] Add mock HTTP responses for API endpoints
- [ ] Test URL redirect handling (HTTP 307)
- [ ] Test multi-instance configuration parsing
- [ ] Validate against real API responses (optional integration tests)
- [ ] Document test coverage in `docs/TEST_SUMMARY.md`

### For Implementation Developers

- [ ] Review remediation strategy in this document
- [ ] Create implementation file: `queue-remediation.go`
- [ ] Implement queue fetching with redirect support
- [ ] Implement status classification logic
- [ ] Implement delete operations with query parameters
- [ ] Implement manual import trigger
- [ ] Add CLI flags for queue remediation mode
- [ ] Update `Makefile` with new build target
- [ ] Update `README.md` with queue remediation section
- [ ] Test against live instances

---

## Production Deployment Notes

**Important Considerations:**

1. **URL Redirects:** Always use HTTP client that follows redirects (Go's `http.Client` does this by default)

2. **Blocklisting:** Using `blocklist=true` prevents future automatic searches from grabbing the same release. Use carefully.

3. **Manual Import:** The manual import command may require additional permissions or trigger user notifications. Test in non-production first.

4. **Polling Frequency:** Queue status syncs with download clients every 30-60 seconds. Don't poll more frequently than this.

5. **Multi-Instance:** Current deployment has 5 instances (3 Sonarr + 2 Radarr). Remediation tool should handle all concurrently.

---

## Next Steps

1. **Test Development** (Priority: High)
   - Create comprehensive test suite covering all scenarios
   - Use mock data from this document
   - Validate against production API structure

2. **Implementation** (Priority: High)
   - Build queue remediation CLI tool
   - Implement classification and remediation logic
   - Add to CalmsToolkit suite

3. **Integration** (Priority: Medium)
   - Consider watch mode for continuous monitoring
   - Add metrics/logging for remediation actions
   - Create automation scripts for scheduled runs

---

## Contact & Support

For questions about this research or implementation:
- Review documentation in `docs/` directory
- Check validation report for live testing details
- Reference CLAUDE.md for development guidelines
- See `.env.example` for configuration format

**Research Status:** Complete ✅  
**Documentation Status:** Validated and Production-Ready ✅  
**Ready for Development:** Yes ✅
