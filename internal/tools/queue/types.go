package queue

import "time"

// StatusMessage represents a status message from the queue API
type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// QueueItem represents an item in the Sonarr/Radarr queue
type QueueItem struct {
	ID                    int             `json:"id"`
	Title                 string          `json:"title"`
	Status                string          `json:"status"`
	TrackedDownloadState  string          `json:"trackedDownloadState"`
	TrackedDownloadStatus string          `json:"trackedDownloadStatus"`
	StatusMessages        []StatusMessage `json:"statusMessages"`
	DownloadClient        string          `json:"downloadClient"`
	DownloadId            string          `json:"downloadId"`
	OutputPath            string          `json:"outputPath"`
	ErrorMessage          string          `json:"errorMessage"`
	Size                  int64           `json:"size"`
	SizeLeft              int64           `json:"sizeleft"`
	TimeLeft              string          `json:"timeleft"`
	Added                 time.Time       `json:"added"`
	ReleaseTitle          string          `json:"releaseTitle"`
	Protocol              string          `json:"protocol"`
	InstanceURL           string          `json:"-"`
	InstanceType          string          `json:"-"`
	InstanceName          string          `json:"-"`
}

// QueueResponse represents the response from the queue API
type QueueResponse struct {
	Page         int         `json:"page"`
	PageSize     int         `json:"pageSize"`
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

// QueueAction represents an action that can be performed on a queue item
type QueueAction string

const (
	ActionDelete       QueueAction = "delete"
	ActionBlocklist    QueueAction = "blocklist"
	ActionRetry        QueueAction = "retry"
	ActionManualImport QueueAction = "manual_import"
	ActionMonitor      QueueAction = "monitor"
)

// QueueReason represents the reason for a queue action
type QueueReason string

const (
	ReasonFailed        QueueReason = "download_failed"
	ReasonCustomFormat  QueueReason = "custom_format_no_upgrade"
	ReasonQuality       QueueReason = "quality_no_upgrade"
	ReasonNoFiles       QueueReason = "no_files_found"
	ReasonSample        QueueReason = "sample_file"
	ReasonIDMatch       QueueReason = "matched_by_id"
	ReasonImportBlocked QueueReason = "import_blocked"
	ReasonDownloading   QueueReason = "downloading_normally"
	ReasonUnknown       QueueReason = "unknown"
)

// QueueItemAction represents the recommended action for a queue item
type QueueItemAction struct {
	Action       QueueAction
	Reason       QueueReason
	Blocklist    bool
	ManualImport bool
}

// View represents the current view state
type View int

const (
	ViewList View = iota
	ViewDetail
	ViewConfirm
)

// Model represents the queue remediation model
type Model struct {
	// Data
	items    []QueueItem
	selected int

	// UI state
	view    View
	loading bool
	error   string
	width   int
	height  int

	// Action state
	pendingAction  QueueAction
	pendingItems   []int
	confirmMessage string

	// Filtering
	filterStatus   string
	showOnlyIssues bool
}
