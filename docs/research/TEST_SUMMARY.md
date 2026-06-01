# CalmsToolkit Test Suite Summary

## Overview

Comprehensive test suites have been implemented for all tools in the CalmsToolkit project. All tests are passing with good coverage of core functionality.

## Test Coverage by Package

| Package | File | Functions | Test Cases | Coverage |
|---------|------|-----------|------------|----------|
| internal/requests | `requests_test.go` | 24 | ~80+ | ~28% |
| internal/streams | `streams_test.go` | 12 | ~30+ | ~42% |
| internal/calendar | `calendar_test.go` | 15 | ~40+ | ~40% |
| internal/feed | `feed_test.go` | 18 | ~40+ | ~54% |
| internal/config | `config_test.go` | 5 | ~20+ | ~65% |
| **Total** | | **74** | **~210+** | |

## Running Tests

### All Test Suites
```bash
make test
```

### With Coverage
```bash
go test -v -cover ./...
```

## Test Coverage by Package

### internal/requests
**Utility Functions:**
- `TestGetYear` - Extract year from media dates (4 cases)
- `TestGetStatusText` - Status text formatting (4 cases)
- `TestFormatDate` - Date formatting (3 cases)

**Configuration:**
- `TestBuildToolConfig` - Config builder (4 cases)

**API Operations:**
- `TestSearchMedia` - TMDB search functionality (3 cases)
- `TestSearchMediaWithSpaces` - Query encoding for searches (3 cases)
- `TestSearchMediaErrorDiagnostics` - Error diagnostics (3 cases)
- `TestGetTVDetails` - TV show details retrieval
- `TestCreateRequest` - Request creation (3 cases)
- `TestGetPendingRequests` - Pending request listing (basic)
- `TestApproveRequest` - Request approval
- `TestDeclineRequest` - Request decline
- `TestTestConnection` - Connection testing (3 cases)
- `TestFetchServiceInstances` - Service instance discovery
- `TestFetchServiceDetails` - Service detail retrieval

**Permission & Count Operations:**
- `TestCheckUserPermissions` - Permission validation (5 cases)
- `TestGetRequestCount` - Request count retrieval (3 cases)

**Pending Requests with Fallback Logic:**
- `TestGetPendingRequestsHappyPath` - Normal filter=pending operation
- `TestGetPendingRequestsNoPending` - Zero pending requests edge case
- `TestGetPendingRequestsPagination` - Multi-page fetching (125 requests)
- `TestGetPendingRequestsWithFallback` - Fallback triggers when count > 0 but filter=pending returns 0
- `TestGetPendingRequestsNoFallbackNeeded` - Fallback doesn't trigger when unnecessary
- `TestGetPendingRequestsFallbackPagination` - Fallback with pagination (125 requests, 3 pages)

**Approval with Overrides:**
- `TestApproveRequestWithOverrides` - Approval with root folder override (5 cases)
- `TestApproveRequestWithOverridesEndpoint` - Verifies correct API endpoints
- `TestApproveRequestWithOverridesNilOverrides` - Nil overrides only calls approve

### internal/streams
**Utility Functions:**
- `TestFormatTimeSince` - Time since formatting (4 cases)
- `TestFormatDuration` - Duration formatting (5 cases)
- `TestGetResolutionName` - Resolution naming (6 cases)
- `TestGenerateSessionID` - Session ID generation

**Configuration:**
- `TestBuildToolConfig` - Config builder
- `TestBuildToolConfigDefaults` - Default and edge cases

**API Operations:**
- `TestFetchPlexStreams` - Plex stream fetching
- `TestFetchJellyfinStreams` - Jellyfin stream fetching

**Data Transformation:**
- `TestPlexVideoToStream` - Plex to Stream conversion
- `TestJellyfinSessionToStream` - Jellyfin to Stream conversion

**Session Management:**
- `TestUpdateHistory` - Session history updates
- `TestGetActiveAndEndedSessions` - Session filtering

### internal/calendar
**Utility Functions:**
- `TestTruncateText` - Text truncation (5 cases)
- `TestCalculateColumnLayout` - Terminal column layout (3 cases)
- `TestCalculateDateRange` - Date range calculation
- `TestGetStatusColor` - Status color determination (3 cases)

**Configuration:**
- `TestBuildToolConfig` - Config builder
- `TestBuildToolConfigNil` - Nil config handling

**API Operations:**
- `TestFetchSonarrCalendar` - Sonarr calendar API
- `TestFetchRadarrCalendar` - Radarr calendar API
- `TestFetchQueue` - Queue API (with issues)

**Calendar Logic:**
- `TestAggregateCalendar` - Calendar aggregation and deduplication
- `TestApplyFilters` - Filter combinations (8 cases)
- `TestBuildDayContentSorting` - Episode sort order
- `TestBuildDayContentTruncation` - Episode truncation for same show
- `TestBuildDayContentMixedTypes` - Episode/movie interleaving

### internal/feed
**Utility Functions:**
- `TestMapSonarrEventType` - Sonarr event mapping (8 cases)
- `TestMapRadarrEventType` - Radarr event mapping (7 cases)
- `TestFormatEpisode` - Episode number formatting (4 cases)
- `TestFormatRelativeTime` - Relative time display (8 cases)
- `TestTruncate` - String truncation (5 cases)
- `TestCenter` - String centering (4 cases)
- `TestGetActionColor` - Action color mapping (8 cases)

**Configuration:**
- `TestBuildToolConfig` - Config builder
- `TestBuildToolConfigNil` - Nil config handling
- `TestBuildToolConfigDefaults` - Default value handling

**Data Transformation:**
- `TestSonarrToHistoryEvent` - Sonarr to event conversion
- `TestRadarrToHistoryEvent` - Radarr to event conversion

**API Operations:**
- `TestFetchSonarrHistory` - Sonarr history API
- `TestFetchRadarrHistory` - Radarr history API
- `TestFetchAllHistory` - Multi-instance fetching
- `TestCustomFormatsInSonarrHistory` - Custom format extraction
- `TestCustomFormatsInRadarrHistory` - Custom format extraction
- `TestFetchAllHistoryErrorHandling` - Partial failure handling

**Event Processing:**
- `TestFilterEvents` - Event type filtering (4 cases)
- `TestEventSortOrder` - Event sort order verification

### internal/config
- `TestDefaultToolkitConfig` - Default values
- `TestConfigSaveLoadRoundTrip` - File I/O round trip
- `TestLoadToolkitConfigFileNotFound` - Missing config handling
- `TestConfigValidate` - Validation rules (7 cases)
- `TestConfigURLNormalization` - Trailing slash stripping
- `TestConfigPath` - Config path construction

## Test Patterns Used

1. **Table-Driven Tests**: Most tests use table-driven approach for multiple test cases
2. **HTTP Mock Servers**: `httptest.NewServer` for API testing
3. **Temporary Directories**: `t.TempDir()` for file I/O tests
4. **Subtests**: `t.Run()` for organized test execution
5. **Error Handling**: Comprehensive error case coverage

## Key Test Achievements

### Overseerr API Bug Workaround

**Problem**: Overseerr API has a known bug where `/request/count` shows pending requests exist, but querying `/request?filter=pending` returns zero results.

**Test Coverage**:
- `TestGetPendingRequestsHappyPath` - Verifies normal operation when filter=pending works
- `TestGetPendingRequestsWithFallback` - Verifies fallback triggers correctly and filters mixed-status results
- `TestGetPendingRequestsNoFallbackNeeded` - Ensures fallback doesn't trigger unnecessarily
- `TestGetPendingRequestsFallbackPagination` - Tests fallback with 125 requests across 3 pages
- `TestGetPendingRequestsPagination` - Tests normal pagination (125 requests, 3 pages)
- `TestGetPendingRequestsNoPending` - Edge case with zero pending requests

## Next Steps

To increase coverage (optional):
1. Add integration tests with real API endpoints (use environment flags)
2. Test CLI argument parsing and validation
3. Add tests for output formatting functions
4. Add benchmark tests for performance-critical functions

## Maintenance

All tests are:
- ✅ Passing consistently
- ✅ Integrated with Makefile (`make test`)
- ✅ Following Go best practices
- ✅ Table-driven where appropriate
- ✅ Properly isolated with mocks
