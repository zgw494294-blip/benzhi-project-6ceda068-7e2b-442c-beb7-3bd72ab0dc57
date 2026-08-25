package observatory

import "time"

type TaskState string

const (
	StateDraft         TaskState = "DRAFT"
	StateCollecting    TaskState = "COLLECTING"
	StateQuarantined   TaskState = "QUARANTINED"
	StateReviewPending TaskState = "REVIEW_PENDING"
	StateFrozen        TaskState = "FROZEN"
	StateReleased      TaskState = "RELEASED"
)

type FindingSeverity string

const (
	SeverityBlocking FindingSeverity = "BLOCKING"
	SeverityWarning  FindingSeverity = "WARNING"
)

type FindingStatus string

const (
	FindingOpen     FindingStatus = "OPEN"
	FindingProposed FindingStatus = "PROPOSED"
	FindingAccepted FindingStatus = "ACCEPTED"
	FindingReturned FindingStatus = "RETURNED"
)

type ArchiveTask struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	InstrumentCode   string    `json:"instrumentCode"`
	ObservationStart time.Time `json:"observationStart"`
	ObservationEnd   time.Time `json:"observationEnd"`
	Owner            string    `json:"owner"`
	State            TaskState `json:"state"`
	Version          int64     `json:"version"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type DatasetRevision struct {
	ID                   string    `json:"id"`
	TaskID               string    `json:"taskId"`
	LogicalPath          string    `json:"logicalPath"`
	ByteSize             int64     `json:"byteSize"`
	MediaType            string    `json:"mediaType"`
	SHA256               string    `json:"sha256"`
	SupersedesRevisionID string    `json:"supersedesRevisionId,omitempty"`
	SubmittedBy          string    `json:"submittedBy"`
	SubmittedAt          time.Time `json:"submittedAt"`
}

type ValidationFinding struct {
	ID             string          `json:"id"`
	TaskID         string          `json:"taskId"`
	RevisionID     string          `json:"revisionId"`
	RuleCode       string          `json:"ruleCode"`
	Severity       FindingSeverity `json:"severity"`
	Message        string          `json:"message"`
	Status         FindingStatus   `json:"status"`
	ResolutionNote string          `json:"resolutionNote,omitempty"`
	ReplacementID  string          `json:"replacementRevisionId,omitempty"`
	ReviewedBy     string          `json:"reviewedBy,omitempty"`
	ReviewedAt     *time.Time      `json:"reviewedAt,omitempty"`
}

type ManifestEntry struct {
	RevisionID  string `json:"revisionId"`
	LogicalPath string `json:"logicalPath"`
	ByteSize    int64  `json:"byteSize"`
	MediaType   string `json:"mediaType"`
	SHA256      string `json:"sha256"`
	EntryDigest string `json:"entryDigest"`
}

type FrozenManifest struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"taskId"`
	TaskVersion int64           `json:"taskVersion"`
	Entries     []ManifestEntry `json:"entries"`
	MerkleRoot  string          `json:"merkleRoot"`
	FrozenBy    string          `json:"frozenBy"`
	FrozenAt    time.Time       `json:"frozenAt"`
}

type ReleaseCredential struct {
	ID               string    `json:"id"`
	TaskID           string    `json:"taskId"`
	ManifestID       string    `json:"manifestId"`
	ManifestRoot     string    `json:"manifestRoot"`
	ApprovedBy       string    `json:"approvedBy"`
	PurposeScope     string    `json:"purposeScope"`
	IssuedAt         time.Time `json:"issuedAt"`
	CredentialDigest string    `json:"credentialDigest"`
}

type AuditEvent struct {
	Sequence      int64     `json:"sequence"`
	TaskID        string    `json:"taskId"`
	TaskVersion   int64     `json:"taskVersion"`
	Action        string    `json:"action"`
	Actor         string    `json:"actor"`
	Role          string    `json:"role"`
	Reason        string    `json:"reason"`
	CorrelationID string    `json:"correlationId"`
	OccurredAt    time.Time `json:"occurredAt"`
}

type Aggregate struct {
	Task       ArchiveTask                  `json:"task"`
	Revisions  map[string]DatasetRevision   `json:"revisions"`
	Findings   map[string]ValidationFinding `json:"findings"`
	Manifest   *FrozenManifest              `json:"manifest,omitempty"`
	Credential *ReleaseCredential           `json:"credential,omitempty"`
}
