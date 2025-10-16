# Overseerr Bug #3949 Fallback Implementation

## Overview

Successfully implemented and tested a comprehensive fallback mechanism to work around Overseerr issue #3949, where the `/request/count` endpoint shows pending requests but `/request?filter=pending` returns zero results.

## Implementation Status

✅ **COMPLETE** - Fallback logic fully implemented and all tests passing (23 test functions, 61 test cases total)

## The Bug

**Issue**: Overseerr API inconsistency
- `/api/v1/request/count` returns `{"pending": N, "approved": M, "total": T}` with N > 0
- `/api/v1/request?filter=pending` returns empty results `[]`
- This prevents users from seeing pending media requests

## The Solution

### Implementation Details

**File**: `media-requests.go` (lines 1265-1436)
**Function**: `getPendingRequests(config Config) ([]MediaRequest, error)`

#### Algorithm

1. **Fetch Count** (`/request/count`)
   - Get expected pending count from API
   - Store as `expectedPendingCount`
   - Used for fallback detection

2. **Primary Fetch** (`/request?filter=pending`)
   - Attempt to fetch with pending filter
   - Uses pagination with `pageSize=50`
   - Accumulates results across pages

3. **Detect Mismatch**
   - Check: `expectedPendingCount > 0 && len(pending) == 0`
   - If true: API bug detected, activate fallback

4. **Fallback Mode** (`/request?filter=all`)
   - Display warning to user with color coding
   - Fetch ALL requests with pagination
   - Same `pageSize=50` as primary fetch
   - Proper skip/take parameter handling

5. **Client-Side Filtering**
   - Iterate through all fetched requests
   - Keep only `status == StatusPending` (value: 1)
   - Return filtered list to user

### Status Constants

```go
const (
    StatusPending  = 1  // Pending approval
    StatusApproved = 2  // Approved
    StatusDeclined = 3  // Declined/Denied
)
```

### Permission Constants

```go
const (
    MANAGE_REQUESTS = 16  // Required permission
    ADMIN           = 2   // Alternative permission
)
```

## Test Coverage

### Test Suite: `media-requests_test.go`

**Total Test Cases**: 61 (23 test functions)
**Status**: ✅ All passing

#### Fallback-Specific Tests

1. **`TestGetPendingRequestsHappyPath`**
   - **Purpose**: Verify normal operation when `filter=pending` works correctly
   - **Setup**: Mock returns 3 pending requests
   - **Verifies**: No fallback triggered, all 3 requests returned with pending status

2. **`TestGetPendingRequestsNoPending`**
   - **Purpose**: Edge case with zero pending requests
   - **Setup**: Count returns 0 pending, filter returns empty
   - **Verifies**: No fallback needed, returns empty list correctly

3. **`TestGetPendingRequestsPagination`**
   - **Purpose**: Verify multi-page fetching works correctly
   - **Setup**: 125 requests across 3 pages (50, 50, 25)
   - **Verifies**: 
     - Makes exactly 3 API calls (skip=0, 50, 100)
     - Returns all 125 requests in correct order
     - Sequential ID ordering maintained

4. **`TestGetPendingRequestsWithFallback`** ⭐
   - **Purpose**: Core fallback logic validation
   - **Setup**: 
     - Count shows 2 pending
     - `filter=pending` returns empty (bug simulation)
     - `filter=all` returns 5 mixed-status requests
   - **Verifies**:
     - Primary fetch attempted first
     - Fallback triggered correctly
     - Client-side filtering works (returns only 2 pending)
     - Mixed statuses properly handled

5. **`TestGetPendingRequestsNoFallbackNeeded`**
   - **Purpose**: Ensure fallback doesn't trigger unnecessarily
   - **Setup**: Count shows 2, filter=pending returns 2
   - **Verifies**: No fallback call made, returns 2 requests

6. **`TestGetPendingRequestsFallbackPagination`** ⭐
   - **Purpose**: Verify fallback handles pagination correctly
   - **Setup**: 
     - Count shows 53 pending
     - `filter=pending` returns empty (bug simulation)
     - `filter=all` returns 125 mixed requests across 3 pages
   - **Verifies**:
     - Fallback makes 3 paginated calls (skip=0, 50, 100)
     - Client-side filtering extracts 28 pending from 125 total
     - All returned requests have pending status
     - ID ordering maintained

#### Supporting Tests

7. **`TestCheckUserPermissions`**
   - Verifies MANAGE_REQUESTS (16) permission
   - Verifies ADMIN (2) permission
   - Tests both permissions together
   - Tests no permissions scenario
   - Tests unauthorized (401) response

8. **`TestGetRequestCount`**
   - Valid count response parsing
   - Zero pending edge case
   - Server error (500) handling

## User Experience

### Normal Operation

```
Fetching: /request?filter=pending&take=50&skip=0
Page 1: Got 3 results (total: 3)
Primary fetch complete: 3 pending requests fetched
Primary fetch successful, no fallback needed.
```

### Fallback Mode (Bug Detected)

```
⚠ WARNING: Overseerr API bug detected!
Expected 2 pending request(s) but filter=pending returned 0 results.
Activating fallback: fetching all requests and filtering client-side...

Fetching: /request?filter=all&take=50&skip=0
Fallback page 1: Got 5 results (total: 5)
Fallback fetch complete: 5 total requests retrieved
Filtering for status=1 (PENDING)...
Client-side filtering complete: 2 pending requests found

✓ Fallback successful: Found 2 pending request(s)
```

### Verbose Diagnostics

When run with `-verbose` flag:

```
=== Diagnostic: Checking pending requests ===
User ID: 1
User Email: admin@example.com
User Permissions: 18
✓ Has MANAGE_REQUESTS permission
Request counts - Pending: 2, Approved: 3, Total: 5
===========================================

Attempting primary fetch with filter=pending...
Fetching: /request?filter=pending&take=50&skip=0
Page 1: Got 0 results (total: 0)
Primary fetch complete: 0 pending requests fetched

[Fallback activation and detailed logging...]
```

## Performance Characteristics

### Best Case (No Bug)
- **API Calls**: 1 count + N pages (filter=pending)
- **Network**: Minimal (only pending requests fetched)
- **Processing**: Direct passthrough of results

### Worst Case (Bug Active)
- **API Calls**: 1 count + 1 failed pending + N pages (filter=all)
- **Network**: Higher (all requests fetched)
- **Processing**: Client-side filtering required
- **Overhead**: ~1-2 extra API calls + filtering time

### Pagination Efficiency

With `pageSize=50`:
- 1-50 requests: 1 page
- 51-100 requests: 2 pages  
- 101-150 requests: 3 pages
- Formula: `ceil(totalRequests / 50)` pages

## Code Quality

✅ **Idiomatic Go**: Follows Go best practices
✅ **Error Handling**: Comprehensive error propagation
✅ **Logging**: Informative user feedback
✅ **Testing**: 61 test cases covering all paths
✅ **Documentation**: Inline comments explain logic
✅ **Maintainable**: Clear structure, easy to debug

## Future Considerations

### When Overseerr Fixes the Bug

1. **Detection Still Works**: Fallback won't trigger if bug is fixed
2. **No Code Changes Needed**: Logic automatically uses primary path
3. **Graceful Degradation**: Fallback remains as safety net
4. **Consider Removal**: After several Overseerr versions without bug

### Potential Enhancements

1. **Cache Count**: Store count result to avoid redundant calls
2. **Partial Fallback**: If some results return, use those first
3. **Metrics**: Track fallback trigger rate for debugging
4. **User Preference**: Allow disabling fallback via flag

## Related Documentation

- `docs/OVERSEERR_API_RESEARCH.md` - API endpoint documentation
- `docs/TEST_SUMMARY.md` - Complete test suite overview
- `EXAMPLES_MEDIA_REQUESTS.md` - Usage examples
- GitHub Issue: https://github.com/sct/overseerr/issues/3949

## Conclusion

The fallback implementation is **production-ready** and provides a robust workaround for Overseerr bug #3949. Users experience minimal disruption, with clear feedback when the fallback activates. The solution is well-tested, performant, and maintainable.
