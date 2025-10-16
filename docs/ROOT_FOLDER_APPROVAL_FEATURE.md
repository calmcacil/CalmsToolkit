# Root Folder Override on Approval - Implementation Summary

## Overview
This document describes the implementation of the root folder override feature for approving media requests in the CalmsToolkit media-requests tool.

## Problem Statement
Users could not set the root folder when approving an Overseerr media request. The existing `approveRequest()` function only approved requests without any parameters, and there was no way to override the root folder during the approval workflow.

## Solution Architecture

### API Endpoints Used
1. **POST /api/v1/request/{requestId}/approve** - Approves the request
2. **PUT /api/v1/request/{requestId}** - Updates the request with rootFolder parameter

### Implementation Strategy
The solution follows a two-step API call pattern:
1. First, approve the request using the existing approval endpoint
2. Then, if a root folder override is specified, update the request with the new root folder

This approach ensures backward compatibility while adding the new functionality.

## New Functions

### 1. `approveRequestWithOverrides(config Config, requestID int, overrides *RequestOverrides) error`

**Purpose**: Approves a request and optionally updates it with a root folder override.

**Logic Flow**:
1. Call `approveRequest()` to approve the request
2. If overrides are nil or RootFolder is empty, return success
3. Otherwise, call PUT endpoint to update the request with the rootFolder
4. Return appropriate error messages if either step fails

**Error Handling**:
- Returns approval errors immediately
- Returns descriptive errors if update fails after successful approval
- Clearly indicates which operation failed in error messages

**Location**: Lines 1454-1485 in media-requests.go

### 2. `selectRootFolderForApproval(config Config, request MediaRequest, reader *bufio.Reader) (*RequestOverrides, error)`

**Purpose**: Interactive prompt that allows users to optionally override the root folder before approving a request.

**Logic Flow**:
1. Determine service type (Radarr for movies, Sonarr for TV shows) from request.Type
2. Fetch available service instances
3. If no services configured, return nil (proceed without overrides)
4. Display prompt asking if user wants to override root folder
5. If user selects "Yes":
   - Show available servers (if multiple)
   - Fetch root folders from selected server
   - Allow user to select a root folder
6. Return selected overrides or nil

**User Options**:
- **[Y]** Yes, select root folder - proceeds to server/folder selection
- **[N]** No, use default - proceeds with approval without overrides
- **[B]** Back - cancels the approval operation

**Graceful Degradation**:
- If service fetch fails, shows error but allows approval to proceed
- If no servers configured, silently proceeds without override option
- If no root folders available, notifies user and proceeds

**Location**: Lines 1487-1664 in media-requests.go

### 3. Updated `handleRequestDetail()` - Case "a" (Approval)

**Changes**:
1. Added call to `selectRootFolderForApproval()` before approval
2. Changed from `approveRequest()` to `approveRequestWithOverrides()`
3. Added display of root folder in success message if override was applied
4. Properly handles cancellation from the root folder selection screen

**Location**: Lines 638-655 in media-requests.go

## User Experience Flow

### Approval Without Override
```
=== Request Details ===
Request ID: 123
Type: Movie
...

Actions:
[A] Approve  [D] Decline  [B] Back

Select action: a

=== Approve Request - Root Folder Override ===
Would you like to override the root folder for this request?
[Y] Yes, select root folder
[N] No, use default (proceed with approval)
[B] Back (cancel approval)

Select option: n

Approving request...
✓ Request approved!
```

### Approval With Override
```
Select action: a

=== Approve Request - Root Folder Override ===
Would you like to override the root folder for this request?
[Y] Yes, select root folder
[N] No, use default (proceed with approval)
[B] Back (cancel approval)

Select option: y

=== Select Radarr Server ===
Available Radarr servers:
1. Radarr (default)
2. Radarr 4K (4K)

Select a server (1-2) or type 'back' to cancel: 2

=== Select Root Folder ===
Server: Radarr 4K

Root folders:
1. /movies/4k
2. /movies/4k-anime

Select a root folder (1-2) or type 'back' to cancel: 1

Approving request...
✓ Request approved!
  Root folder set to: /movies/4k
```

## Testing

### Test Coverage
The implementation includes comprehensive unit tests:

1. **TestApproveRequestWithOverrides** - Tests multiple scenarios:
   - Approval without overrides
   - Approval with empty root folder
   - Approval with root folder override
   - Approval failure
   - Approval succeeds but update fails

2. **TestApproveRequestWithOverridesEndpoint** - Verifies correct API endpoint usage:
   - POST to /api/v1/request/{id}/approve
   - PUT to /api/v1/request/{id}

3. **TestApproveRequestWithOverridesNilOverrides** - Confirms nil overrides only call approve

**Test Results**: All tests pass ✓

**Location**: Lines 1458-1652 in media-requests_test.go

## Code Quality

### Go Best Practices Applied
- ✅ Idiomatic error handling with wrapped errors
- ✅ Clear function documentation (godoc style)
- ✅ Proper separation of concerns
- ✅ Table-driven tests
- ✅ Context preservation in error messages
- ✅ Defensive programming (nil checks, graceful degradation)
- ✅ User-friendly colored terminal output
- ✅ Clear screen management for better UX

### Consistency with Existing Code
- Follows the exact pattern used in `selectRootFolderOverride()` for request creation
- Uses the same color schemes and formatting
- Maintains consistent error handling approach
- Reuses existing helper functions (`fetchServiceInstances()`, `fetchServiceDetails()`)

## Technical Decisions

### Why Two API Calls?
The Overseerr API requires approval via POST to `/approve` endpoint. The PUT endpoint for updating requests doesn't perform approval. Therefore, we need both calls to achieve approval with root folder override.

### Why Optional Override?
Not all users need to override the root folder for every request. The default behavior (pressing Enter or selecting "No") preserves the existing workflow while offering power users the override option when needed.

### Why Clear Screen Between Steps?
The multi-step selection process can be confusing if all prompts remain on screen. Clearing the screen and showing context (server name, request details) helps users stay oriented.

### Error Handling Strategy
- **Non-blocking**: If service fetch fails, we inform the user but allow approval to proceed
- **Descriptive**: Error messages clearly indicate which step failed
- **Recoverable**: Users can cancel at any point and return to the detail view

## Future Enhancements

Potential improvements for future versions:

1. **Remember Last Selection**: Cache the last selected server/folder per user
2. **Server Profiles**: Allow profile override in addition to root folder
3. **Batch Approval**: Support approving multiple requests with the same overrides
4. **Config Defaults**: Allow setting default root folders in .env file
5. **API Optimization**: Investigate if a single API call could accomplish both operations

## Related Files

- **Implementation**: `media-requests.go` (lines 638-655, 1454-1664)
- **Tests**: `media-requests_test.go` (lines 1458-1652)
- **API Research**: `docs/OVERSEERR_API_RESEARCH.md`
- **Examples**: `docs/EXAMPLES_MEDIA_REQUESTS.md`

## Deployment Notes

### Build Instructions
```bash
make build          # Build for current platform
make build-all      # Cross-compile for all platforms
```

### Testing Instructions
```bash
make test                      # Run all tests
go test -tags mediarequests -v  # Verbose test output
```

### No Breaking Changes
This feature is fully backward compatible:
- Existing approval workflow still works (just press 'N' or Enter)
- No configuration changes required
- No database migrations needed
- Existing API calls unchanged

## Conclusion

This implementation successfully adds root folder override capability to the approval workflow while maintaining backward compatibility, following Go best practices, and providing a user-friendly interactive experience. The feature is thoroughly tested and ready for production use.
