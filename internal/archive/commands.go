package archive

import "time"

type CommandMeta struct {
	IdempotencyKey  string `json:"idempotencyKey"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Actor           string `json:"actor"`
	Role            string `json:"role"`
	Reason          string `json:"reason"`
	CorrelationID   string `json:"correlationId"`
}

type CreateTaskCommand struct {
	IdempotencyKey   string    `json:"idempotencyKey"`
	ExpectedVersion  int64     `json:"expectedVersion"`
	Actor            string    `json:"actor"`
	Role             string    `json:"role"`
	Reason           string    `json:"reason"`
	CorrelationID    string    `json:"correlationId"`
	Title            string    `json:"title"`
	InstrumentCode   string    `json:"instrumentCode"`
	ObservationStart time.Time `json:"observationStart"`
	ObservationEnd   time.Time `json:"observationEnd"`
	Owner            string    `json:"owner"`
}

type RegisterRevisionCommand struct {
	CommandMeta
	LogicalPath          string `json:"logicalPath"`
	ByteSize             int64  `json:"byteSize"`
	MediaType            string `json:"mediaType"`
	SHA256               string `json:"sha256"`
	SupersedesRevisionID string `json:"supersedesRevisionId"`
}

type ValidateCommand struct {
	CommandMeta
}

type ProposeResolutionCommand struct {
	CommandMeta
	FindingID             string `json:"findingId"`
	ReplacementRevisionID string `json:"replacementRevisionId"`
	ResolutionNote        string `json:"resolutionNote"`
}

type ReviewResolutionCommand struct {
	CommandMeta
	FindingID  string `json:"findingId"`
	Accepted   bool   `json:"accepted"`
	ReviewNote string `json:"reviewNote"`
}

type FreezeCommand struct {
	CommandMeta
}

type IssueCredentialCommand struct {
	CommandMeta
	PurposeScope string `json:"purposeScope"`
}
