package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrProtocol   = errors.New("adapter protocol error")
	commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type terminalPayload struct {
	SessionID    string `json:"sessionId"`
	ResultCommit string `json:"resultCommit"`
}

type RepairFunc func(context.Context, string) (string, error)

func ParseStructuredResult(input string) (Result, error) {
	for index := 0; index < len(input); index++ {
		if input[index] != '{' {
			continue
		}
		reader := strings.NewReader(input[index:])
		decoder := json.NewDecoder(reader)
		decoder.DisallowUnknownFields()
		var payload terminalPayload
		if err := decoder.Decode(&payload); err != nil {
			continue
		}
		if payload.SessionID == "" || !commitPattern.MatchString(payload.ResultCommit) {
			continue
		}
		length := int(decoder.InputOffset())
		raw := json.RawMessage(append([]byte(nil), input[index:index+length]...))
		return Result{
			SessionID:       payload.SessionID,
			ResultCommit:    payload.ResultCommit,
			StructuredValue: raw,
		}, nil
	}
	return Result{}, fmt.Errorf("%w: no schema-valid terminal JSON object", ErrProtocol)
}

func ParseStructuredResultWithRepair(
	ctx context.Context,
	input string,
	repair RepairFunc,
) (Result, error) {
	result, err := ParseStructuredResult(input)
	if err == nil {
		return result, nil
	}
	if repair == nil {
		return Result{}, err
	}
	repaired, repairErr := repair(ctx, input)
	if repairErr != nil {
		return Result{}, fmt.Errorf("%w: repair failed: %v", ErrProtocol, repairErr)
	}
	result, err = ParseStructuredResult(repaired)
	if err != nil {
		return Result{}, fmt.Errorf("%w: repaired output invalid", ErrProtocol)
	}
	return result, nil
}
