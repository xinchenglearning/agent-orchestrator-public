package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/adapters/codex"
	"github.com/xinchenglearning/agent-orchestrator/internal/domain"
	"github.com/xinchenglearning/agent-orchestrator/internal/host"
	"github.com/xinchenglearning/agent-orchestrator/internal/orchestrator"
	"github.com/xinchenglearning/agent-orchestrator/internal/policy"
	"github.com/xinchenglearning/agent-orchestrator/internal/redaction"
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

func execute(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: orch <prepare|run|status|decide>")
		return 2
	}
	var err error
	switch args[0] {
	case "prepare":
		err = prepareCommand(args[1:], stdout, stderr)
	case "run":
		err = runCommand(args[1:], stdout, stderr)
	case "status":
		err = statusCommand(args[1:], stdout, stderr)
	case "decide":
		err = decideCommand(args[1:], stdout, stderr)
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func prepareCommand(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("prepare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	taskPath := flags.String("task", "", "task JSON path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	task, err := loadTask(*taskPath)
	if err != nil {
		return err
	}
	prepared, digest, err := (host.Service{}).Prepare(context.Background(), task)
	if err != nil {
		return err
	}
	return writeJSON(stdout, struct {
		Task   domain.Task `json:"task"`
		Digest string      `json:"digest"`
	}{prepared, digest})
}

func runCommand(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	taskPath := flags.String("task", "", "task JSON path")
	approvedDigest := flags.String("approved-task-digest", "", "approved task digest")
	strategy := flags.String("strategy", "single", "execution strategy")
	writer := flags.String("writer", "codex", "writer adapter")
	modeValue := flags.String("mode", string(policy.ExecutionTrustedLocal), "execution mode")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *strategy != "single" {
		return fmt.Errorf("unsupported strategy %q", *strategy)
	}
	if *writer != "codex" {
		return fmt.Errorf("unsupported writer %q", *writer)
	}
	mode := policy.ExecutionMode(*modeValue)
	if mode != policy.ExecutionTrustedLocal && mode != policy.ExecutionStrict {
		return fmt.Errorf("unsupported mode %q", mode)
	}
	task, err := loadTask(*taskPath)
	if err != nil {
		return err
	}
	service := host.Service{Engine: orchestrator.Engine{
		Adapter:  codex.Default(),
		Mode:     mode,
		Redactor: redaction.New(authenticationSecrets()),
	}}
	prepared, _, err := service.Prepare(context.Background(), task)
	if err != nil {
		return err
	}
	run, err := service.Run(context.Background(), prepared, *approvedDigest)
	if err != nil {
		return err
	}
	return writeJSON(stdout, run)
}

func statusCommand(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("status requires a run ID")
	}
	runID := args[0]
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repository := flags.String("repository", ".", "repository root")
	_ = flags.Bool("json", true, "emit JSON")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	run, err := (host.Service{Engine: orchestrator.Engine{}}).Status(
		context.Background(),
		*repository,
		runID,
	)
	if err != nil {
		return err
	}
	return writeJSON(stdout, run)
}

func decideCommand(args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		return errors.New("decide requires a run ID and decision")
	}
	runID := args[0]
	decision := domain.HumanDecision(args[1])
	flags := flag.NewFlagSet("decide", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repository := flags.String("repository", ".", "repository root")
	packetDigest := flags.String(
		"decision-packet-digest",
		"",
		"approved decision packet digest",
	)
	actor := flags.String("actor", os.Getenv("USER"), "human actor")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	run, err := (host.Service{Engine: orchestrator.Engine{}}).Decide(
		context.Background(),
		*repository,
		runID,
		domain.Approval{
			Actor:                *actor,
			Decision:             decision,
			TaskDigest:           statusTaskDigest(*repository, runID),
			DecisionPacketDigest: *packetDigest,
			At:                   time.Now(),
		},
	)
	if err != nil {
		return err
	}
	return writeJSON(stdout, run)
}

func statusTaskDigest(repository, runID string) string {
	run, err := (host.Service{Engine: orchestrator.Engine{}}).Status(
		context.Background(),
		repository,
		runID,
	)
	if err != nil {
		return ""
	}
	return run.TaskDigest
}

func loadTask(path string) (domain.Task, error) {
	if path == "" {
		return domain.Task{}, errors.New("--task is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return domain.Task{}, fmt.Errorf("open task: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var task domain.Task
	if err := decoder.Decode(&task); err != nil {
		return domain.Task{}, fmt.Errorf("decode task: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return domain.Task{}, errors.New("task JSON must contain one object")
	}
	return task, nil
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func authenticationSecrets() []string {
	var secrets []string
	for _, key := range []string{"OPENAI_API_KEY", "CODEX_API_KEY"} {
		if value, ok := os.LookupEnv(key); ok {
			secrets = append(secrets, value)
		}
	}
	return secrets
}
