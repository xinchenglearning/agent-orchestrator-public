package domain_test

import (
	"testing"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
)

func TestTransitionFollowsHumanGatedPath(t *testing.T) {
	tests := []struct {
		event domain.EventType
		want  domain.RunState
	}{
		{domain.EventPreparationStarted, domain.RunPreparing},
		{domain.EventImplementationStarted, domain.RunImplementing},
		{domain.EventImplementationFinished, domain.RunVerifying},
		{domain.EventVerificationFinished, domain.RunAwaitingApproval},
		{domain.EventApproved, domain.RunCompleted},
	}

	state := domain.RunCreated
	for _, tt := range tests {
		next, err := domain.Transition(state, tt.event)
		if err != nil {
			t.Fatalf("%s + %s: %v", state, tt.event, err)
		}
		if next != tt.want {
			t.Fatalf("%s + %s: got %s, want %s", state, tt.event, next, tt.want)
		}
		state = next
	}
}

func TestTransitionSupportsBoundedHumanDecisions(t *testing.T) {
	tests := []struct {
		name  string
		event domain.EventType
		want  domain.RunState
	}{
		{"approve", domain.EventApproved, domain.RunCompleted},
		{"request rework", domain.EventReworkRequested, domain.RunReworking},
		{"reject", domain.EventRejected, domain.RunRejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.Transition(domain.RunAwaitingApproval, tt.event)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}

	got, err := domain.Transition(domain.RunReworking, domain.EventImplementationStarted)
	if err != nil {
		t.Fatal(err)
	}
	if got != domain.RunImplementing {
		t.Fatalf("got %s, want %s", got, domain.RunImplementing)
	}
}

func TestTransitionRejectsCompletionBeforeApproval(t *testing.T) {
	got, err := domain.Transition(domain.RunVerifying, domain.EventApproved)
	if err == nil {
		t.Fatal("expected invalid transition")
	}
	if got != domain.RunVerifying {
		t.Fatalf("invalid transition changed state to %s", got)
	}
}

func TestTransitionHandlesCancellationAndAmbiguousWrites(t *testing.T) {
	for _, state := range []domain.RunState{
		domain.RunCreated,
		domain.RunPreparing,
		domain.RunImplementing,
		domain.RunVerifying,
		domain.RunAwaitingApproval,
		domain.RunReworking,
	} {
		got, err := domain.Transition(state, domain.EventCanceled)
		if err != nil {
			t.Fatalf("cancel %s: %v", state, err)
		}
		if got != domain.RunCanceled {
			t.Fatalf("cancel %s: got %s", state, got)
		}
	}

	got, err := domain.Transition(domain.RunImplementing, domain.EventWriteAmbiguous)
	if err != nil {
		t.Fatal(err)
	}
	if got != domain.RunRecoveryRequired {
		t.Fatalf("got %s, want %s", got, domain.RunRecoveryRequired)
	}

	got, err = domain.Transition(domain.RunVerifying, domain.EventFailed)
	if err != nil {
		t.Fatal(err)
	}
	if got != domain.RunFailed {
		t.Fatalf("got %s, want %s", got, domain.RunFailed)
	}
}

func TestValidateApprovalBindsPersistedDigests(t *testing.T) {
	approval := domain.Approval{
		Actor:                "maintainer@example.com",
		Decision:             domain.DecisionApprove,
		TaskDigest:           "task-digest",
		DecisionPacketDigest: "packet-digest",
		At:                   time.Now(),
	}
	if err := domain.ValidateApproval(approval, "task-digest", "packet-digest"); err != nil {
		t.Fatal(err)
	}

	approval.DecisionPacketDigest = "stale-packet"
	if err := domain.ValidateApproval(approval, "task-digest", "packet-digest"); err == nil {
		t.Fatal("expected stale approval rejection")
	}
}
