package adapters

import (
	"context"
	"encoding/json"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/policy"
)

type Capabilities struct {
	StructuredOutput bool
	Cancellation     bool
	SessionResume    bool
	ReadOnly         bool
	WorkspaceWrite   bool
	NativeSandbox    bool
	Streaming        bool
}

type Adapter interface {
	Probe(context.Context) (Capabilities, error)
	Start(context.Context, Job) (Session, error)
}

type Session interface {
	Events() <-chan Event
	Wait(context.Context) (Result, error)
	Cancel(context.Context) error
}

type Event struct {
	Type       string          `json:"type"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data,omitempty"`
	At         time.Time       `json:"at"`
	Generation uint64          `json:"generation"`
}

type Result struct {
	SessionID       string          `json:"sessionId"`
	ResultCommit    string          `json:"resultCommit"`
	StructuredValue json.RawMessage `json:"structuredValue,omitempty"`
	Usage           Usage           `json:"usage"`
}

type Usage struct {
	InputTokens  int64         `json:"inputTokens"`
	OutputTokens int64         `json:"outputTokens"`
	WallTime     time.Duration `json:"wallTime"`
}

type Job struct {
	Task          domain.Task
	TaskDigest    string
	Role          string
	ExecutionMode policy.ExecutionMode
	WorkspaceDir  string
	EvidenceDir   string
	Budget        domain.Budget
	Generation    uint64
}
