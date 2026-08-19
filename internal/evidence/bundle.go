package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/xinchenglearning/agent-orchestrator/internal/verification"
)

type Artifact struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type EvidenceBundle struct {
	RunID         string                       `json:"runId"`
	TaskDigest    string                       `json:"taskDigest"`
	ExecutionMode string                       `json:"executionMode"`
	BaseCommit    string                       `json:"baseCommit"`
	ResultCommit  string                       `json:"resultCommit"`
	Artifacts     []Artifact                   `json:"artifacts"`
	Verification  []verification.CommandResult `json:"verification"`
}

func ArtifactFromBytes(name string, data []byte) Artifact {
	sum := sha256.Sum256(data)
	return Artifact{
		Name:   name,
		SHA256: hex.EncodeToString(sum[:]),
		Size:   int64(len(data)),
	}
}

func Digest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal digest value: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
