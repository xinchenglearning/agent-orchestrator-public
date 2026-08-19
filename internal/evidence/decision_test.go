package evidence_test

import (
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/evidence"
)

func TestDecisionPacketReferencesImmutableDigests(t *testing.T) {
	packet := evidence.DecisionPacket{
		TaskDigest:         "task",
		EvidenceDigest:     "evidence",
		ReviewDigest:       "review",
		VerificationDigest: "verification",
	}
	first, err := evidence.Digest(packet)
	if err != nil {
		t.Fatal(err)
	}
	packet.EvidenceDigest = "changed"
	second, err := evidence.Digest(packet)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("decision digest did not change")
	}
}
