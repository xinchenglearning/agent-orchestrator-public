package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type RunState string

const (
	RunCreated          RunState = "CREATED"
	RunPreparing        RunState = "PREPARING"
	RunImplementing     RunState = "IMPLEMENTING"
	RunVerifying        RunState = "VERIFYING"
	RunAwaitingApproval RunState = "AWAITING_APPROVAL"
	RunReworking        RunState = "REWORKING"
	RunCompleted        RunState = "COMPLETED"
	RunRejected         RunState = "REJECTED"
	RunFailed           RunState = "FAILED"
	RunCanceled         RunState = "CANCELED"
	RunRecoveryRequired RunState = "RECOVERY_REQUIRED"
)

type HumanDecision string

const (
	DecisionApprove       HumanDecision = "approve"
	DecisionRequestRework HumanDecision = "request_rework"
	DecisionReject        HumanDecision = "reject"
	DecisionCancel        HumanDecision = "cancel"
)

type Approval struct {
	Actor                string        `json:"actor"`
	Decision             HumanDecision `json:"decision"`
	TaskDigest           string        `json:"taskDigest"`
	DecisionPacketDigest string        `json:"decisionPacketDigest"`
	At                   time.Time     `json:"at"`
}

type EventType string

const (
	EventPreparationStarted     EventType = "preparation_started"
	EventImplementationStarted  EventType = "implementation_started"
	EventImplementationFinished EventType = "implementation_finished"
	EventVerificationFinished   EventType = "verification_finished"
	EventApproved               EventType = "approved"
	EventReworkRequested        EventType = "rework_requested"
	EventRejected               EventType = "rejected"
	EventCanceled               EventType = "canceled"
	EventFailed                 EventType = "failed"
	EventWriteAmbiguous         EventType = "write_ambiguous"
)

type transitionKey struct {
	state RunState
	event EventType
}

var transitions = map[transitionKey]RunState{
	{RunCreated, EventPreparationStarted}:          RunPreparing,
	{RunPreparing, EventImplementationStarted}:     RunImplementing,
	{RunImplementing, EventImplementationFinished}: RunVerifying,
	{RunVerifying, EventVerificationFinished}:      RunAwaitingApproval,
	{RunAwaitingApproval, EventApproved}:           RunCompleted,
	{RunAwaitingApproval, EventReworkRequested}:    RunReworking,
	{RunAwaitingApproval, EventRejected}:           RunRejected,
	{RunReworking, EventImplementationStarted}:     RunImplementing,
}

func Transition(current RunState, event EventType) (RunState, error) {
	if isTerminal(current) {
		return current, fmt.Errorf("run state %s is terminal", current)
	}
	if event == EventCanceled {
		return RunCanceled, nil
	}
	if event == EventFailed {
		return RunFailed, nil
	}
	if event == EventWriteAmbiguous {
		return RunRecoveryRequired, nil
	}
	next, ok := transitions[transitionKey{current, event}]
	if !ok {
		return current, fmt.Errorf("invalid transition: %s + %s", current, event)
	}
	return next, nil
}

func ValidateApproval(
	approval Approval,
	expectedTaskDigest string,
	expectedDecisionPacketDigest string,
) error {
	if strings.TrimSpace(approval.Actor) == "" {
		return errors.New("approval actor is required")
	}
	if approval.At.IsZero() {
		return errors.New("approval time is required")
	}
	switch approval.Decision {
	case DecisionApprove, DecisionRequestRework, DecisionReject, DecisionCancel:
	default:
		return fmt.Errorf("unknown human decision %q", approval.Decision)
	}
	if approval.TaskDigest == "" || approval.TaskDigest != expectedTaskDigest {
		return errors.New("approval task digest does not match")
	}
	if approval.DecisionPacketDigest == "" ||
		approval.DecisionPacketDigest != expectedDecisionPacketDigest {
		return errors.New("approval decision packet digest does not match")
	}
	return nil
}

func isTerminal(state RunState) bool {
	switch state {
	case RunCompleted, RunRejected, RunFailed, RunCanceled, RunRecoveryRequired:
		return true
	default:
		return false
	}
}
