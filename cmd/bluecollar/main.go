package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yeomyeonggeori/bluecollar/agentcontract"
	"github.com/yeomyeonggeori/bluecollar/intake"
	"github.com/yeomyeonggeori/bluecollar/loop"
	"github.com/yeomyeonggeori/bluecollar/model/openaicompatible"
	"github.com/yeomyeonggeori/bluecollar/taskstate"
	"github.com/yeomyeonggeori/bluecollar/toolcontract"
)

func main() {
	endpointURL := flag.String("endpoint", envOrDefault("BLUECOLLAR_MODEL_ENDPOINT", "http://127.0.0.1:11434/v1"), "OpenAI-compatible base URL")
	apiKey := flag.String("api-key", os.Getenv("BLUECOLLAR_MODEL_API_KEY"), "bearer token for the endpoint, when it needs one")
	modelName := flag.String("model", envOrDefault("BLUECOLLAR_MODEL", "qwen3"), "model to ask")
	agentName := flag.String("agent-name", "the assistant", "what the agent calls itself")
	timeout := flag.Duration("timeout", 5*time.Minute, "how long one turn may run")
	workspacePath := flag.String("workspace", ".", "directory the agent's shell commands run in")
	withoutTools := flag.Bool("without-tools", false, "answer from reasoning alone, giving the agent no shell")
	flag.Parse()

	prompt := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "usage: bluecollar [flags] <what you want done>")
		os.Exit(2)
	}

	result, errorValue := runOneTurn(runOptions{
		endpointURL:   *endpointURL,
		apiKey:        *apiKey,
		modelName:     *modelName,
		agentName:     *agentName,
		prompt:        prompt,
		timeout:       *timeout,
		workspacePath: *workspacePath,
		withoutTools:  *withoutTools,
	})
	if errorValue != nil {
		fmt.Fprintln(os.Stderr, "bluecollar:", errorValue)
		os.Exit(1)
	}
	printResult(result)
}

type runOptions struct {
	endpointURL   string
	apiKey        string
	modelName     string
	agentName     string
	prompt        string
	timeout       time.Duration
	workspacePath string
	withoutTools  bool
}

func runOneTurn(options runOptions) (agentcontract.AgentTurnResult, error) {
	languageModel := openaicompatible.NewProvider(options.endpointURL, options.apiKey, options.modelName)
	taskEventService := taskstate.NewTaskEventService()
	taskRunService := taskstate.NewTaskRunService(taskEventService)
	kernel := loop.NewAgentKernel(taskRunService, taskstate.NewTaskStepService())
	kernel.UseLanguageModelProvider(languageModel)

	turnContext, cancel := context.WithTimeout(context.Background(), options.timeout)
	defer cancel()

	request := agentcontract.AgentTurnRequest{
		RequesterPersonID: "person-local",
		RequesterName:     currentUserName(),
		ConversationID:    "conversation-local",
		Prompt:            options.prompt,
		AgentIdentity:     agentcontract.AgentIdentity{Name: options.agentName},
		WorkspaceRootPath: options.workspacePath,
		ToolSet:           turnToolSet(options),
	}

	turnDecision, errorValue := routeTurn(turnContext, languageModel, request)
	if errorValue != nil {
		return agentcontract.AgentTurnResult{}, errorValue
	}
	request.PrecomputedTurnDecision = &turnDecision

	result, errorValue := kernel.RunTurn(turnContext, request)
	printLedger(taskRunService, result.TaskRun.TaskRunID)
	return result, errorValue
}

func printLedger(taskRunService *taskstate.TaskRunService, taskRunID string) {
	if strings.TrimSpace(taskRunID) == "" {
		return
	}
	for _, taskEvent := range taskRunService.ListTaskEvent(taskRunID) {
		fmt.Fprintf(os.Stderr, "  %s  %s\n", taskEvent.Name, truncated(taskEvent.Body))
	}
}

func truncated(text string) string {
	const limit = 160
	collapsed := strings.Join(strings.Fields(text), " ")
	if len(collapsed) <= limit {
		return collapsed
	}
	return collapsed[:limit] + "…"
}

func routeTurn(ctx context.Context, languageModel *openaicompatible.Provider, request agentcontract.AgentTurnRequest) (agentcontract.TurnDecision, error) {
	router := intake.NewTurnRouter(languageModel, agentcontract.IntakeOptions{IsEnabled: true})
	return router.Plan(ctx, agentcontract.AgentRequest{
		RequesterPersonID: request.RequesterPersonID,
		RequesterName:     request.RequesterName,
		ConversationID:    request.ConversationID,
		Prompt:            request.Prompt,
	})
}

func printResult(result agentcontract.AgentTurnResult) {
	fmt.Println(strings.TrimSpace(firstNonEmpty(result.FinishMessage, result.UserNotice, "(no reply)")))
	fmt.Fprintf(os.Stderr, "\nstatus: %s\n", result.TaskRun.Status)
	if reason := strings.TrimSpace(result.TaskRun.FailureReason); reason != "" {
		fmt.Fprintln(os.Stderr, "reason:", reason)
	}
}

func currentUserName() string {
	if name := strings.TrimSpace(os.Getenv("USER")); name != "" {
		return name
	}
	return "local"
}

func envOrDefault(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func turnToolSet(options runOptions) *toolcontract.ToolSet {
	if options.withoutTools {
		return nil
	}
	return newWorkspaceToolSet(options.workspacePath)
}
