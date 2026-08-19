package evidence_test

import (
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/evidence"
)

func TestEvidenceDigestChangesWithReferencedArtifact(t *testing.T) {
	firstArtifact := evidence.ArtifactFromBytes("diff.patch", []byte("first"))
	first := evidence.EvidenceBundle{
		RunID:         "run-1",
		TaskDigest:    "task-digest",
		ExecutionMode: "trusted-local",
		BaseCommit:    "base",
		ResultCommit:  "result",
		Artifacts:     []evidence.Artifact{firstArtifact},
	}
	firstDigest, err := evidence.Digest(first)
	if err != nil {
		t.Fatal(err)
	}

	second := first
	second.Artifacts = []evidence.Artifact{
		evidence.ArtifactFromBytes("diff.patch", []byte("changed")),
	}
	secondDigest, err := evidence.Digest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest == secondDigest {
		t.Fatal("bundle digest did not change with artifact")
	}
}
