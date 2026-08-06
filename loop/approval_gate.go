package loop

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/yeomyeonggeori/bluecollar/toolcontract"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/bluecollar/model"
	"github.com/yeomyeonggeori/bluecollar/taskstate"
)

type approvalHeldCall struct {
	ToolName     string          `json:"toolName"`
	ToolInput    json.RawMessage `json:"toolInput"`
	Confirmation string          `json:"confirmation"`
}

func isApprovalRequiredObservation(observation turnObservation) bool {
	return observation.Failed() && observation.Failure.RequiresApproval
}

func (agentTurnRunner *AgentTurnRunner) requestHeldCallApproval(ctx context.Context, taskRunID string, stepID string, request AgentTurnRequest, state *agentTaskState, actionDocument turnActionDocument) toolCallActionOutcome {
	confirmation, errorValue := agentTurnRunner.heldCallConfirmationWording(ctx, request, actionDocument)
	if errorValue != nil {
		result, _ := agentTurnRunner.failTurn(taskRunID, request, errorValue.Error(), state.Observations, state.Attachments, state.ExecutionState)
		return toolCallActionOutcome{Result: result, ShouldReturn: true, WasHandled: true}
	}
	heldCall := approvalHeldCall{
		ToolName:     strings.TrimSpace(actionDocument.ToolName),
		ToolInput:    copyJSONRawMessage(actionDocument.ToolInput),
		Confirmation: confirmation,
	}
	agentTurnRunner.appendEvent(taskRunID, "approval.pending_call", marshalEventBody(heldCall))
	pausedTaskRun, errorValue := agentTurnRunner.taskRunService.PauseTaskRun(taskRunID, taskstate.TaskStatusWaitingApproval, confirmation)
	if errorValue != nil {
		result, _ := agentTurnRunner.failTurn(taskRunID, request, errorValue.Error(), state.Observations, state.Attachments, state.ExecutionState)
		return toolCallActionOutcome{Result: result, ShouldReturn: true, WasHandled: true}
	}
	agentTurnRunner.appendEvent(taskRunID, "confirmation.requested", marshalEventBody(map[string]string{
		"userFacingMessage": confirmation,
		"message":           confirmation,
		"reasonCode":        "external_send",
		"reasonDetail":      "runtime approval gate for " + heldCall.ToolName,
		"responseLanguage":  request.ResponseLanguage,
	}))
	askBody := map[string]any{
		"kind":             "ask_confirm",
		"message":          confirmation,
		"reasonCode":       "external_send",
		"reasonDetail":     "runtime approval gate for " + heldCall.ToolName,
		"responseLanguage": request.ResponseLanguage,
	}
	// A tool that belongs to an approval family can be approved for the rest of the
	// task instead of once, so the user is not asked again for every step of the
	// same piece of work.
	if scope := approvalScopeForTool(request.ToolSet, heldCall.ToolName); scope != "" {
		askBody["approvalScope"] = scope
		askBody["sessionApprovable"] = true
		askBody["actions"] = []string{"confirm", "confirm_task", "cancel"}
	}
	agentTurnRunner.appendEvent(taskRunID, "ask.requested", marshalEventBody(askBody))
	agentTurnRunner.saveStep(taskRunID, stepID, taskstate.TaskStatusWaitingApproval, "approval "+heldCall.ToolName, confirmation)
	return toolCallActionOutcome{
		Result:       AgentTurnResult{TaskRun: pausedTaskRun, UserNotice: confirmation, Attachments: state.Attachments},
		ShouldReturn: true,
		WasHandled:   true,
	}
}

func approvalScopeForTool(toolSet *toolcontract.ToolSet, toolName string) string {
	definition, isFound := toolSet.ToolDefinition(strings.TrimSpace(toolName))
	if !isFound {
		return ""
	}
	return strings.TrimSpace(definition.ApprovalScope)
}

func (agentTurnRunner *AgentTurnRunner) taskAlreadyApprovedScope(taskRunID string, toolSet *toolcontract.ToolSet, toolName string) bool {
	scope := approvalScopeForTool(toolSet, toolName)
	if scope == "" {
		return false
	}
	return taskApprovedScopes(agentTurnRunner.taskRunService.ListTaskEvent(taskRunID))[scope]
}

func taskApprovedScopes(taskEvents []taskstate.TaskEvent) map[string]bool {
	approvedScopes := map[string]bool{}
	for _, taskEvent := range taskEvents {
		if taskEvent.Name != "approval.scope_granted" {
			continue
		}
		var body struct {
			Scope string `json:"scope"`
		}
		if json.Unmarshal([]byte(taskEvent.Body), &body) != nil {
			continue
		}
		if scope := strings.TrimSpace(body.Scope); scope != "" {
			approvedScopes[scope] = true
		}
	}
	return approvedScopes
}

func toolCallRequiresRuntimeApproval(toolSet *toolcontract.ToolSet, actionDocument turnActionDocument) bool {
	trimmedToolName := strings.TrimSpace(actionDocument.ToolName)
	if trimmedToolName == "" {
		return false
	}
	definition, isFound := toolSet.ToolDefinition(trimmedToolName)
	if isFound && definition.RequiresApproval {
		return !sendHasReplyBlastRadius(definition, actionDocument.ToolInput)
	}
	if trimmedToolName != toolcontract.TerminalRunToolName {
		return false
	}
	var input struct {
		ApprovalRequired bool `json:"approvalRequired"`
	}
	return json.Unmarshal(actionDocument.ToolInput, &input) == nil && input.ApprovalRequired
}

func sendHasReplyBlastRadius(definition toolcontract.ToolDefinition, toolInput json.RawMessage) bool {
	return toolcontract.ToolDefinitionSideEffectClass(definition) == toolcontract.ToolSideEffectExternalSend &&
		sendTargetsCurrentConversation(toolInput)
}

type approvalQuestionContextDocument struct {
	ResponseLanguage string            `json:"responseLanguage,omitempty"`
	OriginalRequest  string            `json:"originalRequest,omitempty"`
	ModelDraft       string            `json:"modelDraft,omitempty"`
	Operation        string            `json:"operation,omitempty"`
	ActionDetails    map[string]string `json:"actionDetails,omitempty"`
}

type approvalQuestionResponseDocument struct {
	Question string `json:"question"`
}

type approvalQuestionActionInput struct {
	TargetType     string   `json:"targetType"`
	PersonHint     string   `json:"personHint"`
	ChannelName    string   `json:"channelName"`
	Message        string   `json:"message"`
	Body           string   `json:"body"`
	Subject        string   `json:"subject"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	Reason         string   `json:"reason"`
	ApprovalReason string   `json:"approvalReason"`
	Path           string   `json:"path"`
	DevicePath     string   `json:"devicePath"`
	TargetPath     string   `json:"targetPath"`
	Slug           string   `json:"slug"`
	SiteID         string   `json:"siteID"`
	EventHint      string   `json:"eventHint"`
	To             []string `json:"to"`
	People         []string `json:"people"`
}

func (agentTurnRunner *AgentTurnRunner) heldCallConfirmationWording(ctx context.Context, request AgentTurnRequest, actionDocument turnActionDocument) (string, error) {
	modelDraft := deliverableModelWording(actionDocument.Message)
	return agentTurnRunner.generateHeldCallConfirmationWording(ctx, request, actionDocument, modelDraft)
}

func (agentTurnRunner *AgentTurnRunner) generateHeldCallConfirmationWording(ctx context.Context, request AgentTurnRequest, actionDocument turnActionDocument, modelDraft string) (string, error) {
	if agentTurnRunner.languageModel == nil {
		return "", errors.New("language model provider is not configured")
	}
	contextDocument := approvalQuestionContext(request, actionDocument, modelDraft)
	contextDocumentBytes, errorValue := json.Marshal(contextDocument)
	if errorValue != nil {
		return "", errorValue
	}
	structuredResponse, errorValue := agentTurnRunner.languageModel.GenerateStructuredResponse(ctx, model.StructuredResponseRequest{
		Messages: []model.Message{
			{Role: "system", Content: strings.Join([]string{
				"Write exactly one concise user-facing approval question.",
				"The question asks whether to perform the pending action.",
				"Use the original request, model draft, and action details to phrase the target, content, file, event, or site naturally.",
				"Include consequential details when present so the user can approve a concrete action.",
				"Do not mention internal tool names, operation identifiers, JSON, schemas, approval gates, runtime, or implementation details.",
				"Do not answer the question, report status, or explain the policy.",
			}, "\n")},
			{Role: "system", Content: responseLanguageInstruction(request.ResponseLanguage)},
			{Role: "user", Content: string(contextDocumentBytes)},
		},
		StructuredOutputSchema: model.StructuredOutputSchema{
			Name:               "bluecollar_approval_question",
			Document:           `{"type":"object","properties":{"question":{"type":"string"}},"required":["question"],"additionalProperties":false}`,
			IsStrictlyEnforced: true,
		},
	})
	if errorValue != nil {
		return "", errorValue
	}
	var responseDocument approvalQuestionResponseDocument
	if errorValue := json.Unmarshal([]byte(structuredResponse.Content), &responseDocument); errorValue != nil {
		return "", errorValue
	}
	question := strings.TrimSpace(responseDocument.Question)
	if question == "" {
		return "", errors.New("approval question is empty")
	}
	return question, nil
}

func approvalQuestionContext(request AgentTurnRequest, actionDocument turnActionDocument, modelDraft string) approvalQuestionContextDocument {
	return approvalQuestionContextDocument{
		ResponseLanguage: strings.TrimSpace(request.ResponseLanguage),
		OriginalRequest:  strings.TrimSpace(request.Prompt),
		ModelDraft:       strings.TrimSpace(modelDraft),
		Operation:        strings.TrimSpace(actionDocument.ToolName),
		ActionDetails:    approvalQuestionActionDetails(actionDocument.ToolInput),
	}
}

func approvalQuestionActionDetails(input json.RawMessage) map[string]string {
	if len(input) == 0 {
		return nil
	}
	var document approvalQuestionActionInput
	if json.Unmarshal(input, &document) != nil {
		return nil
	}
	details := map[string]string{}
	approvalQuestionSetDetail(details, "target", firstNonEmptyString(document.PersonHint, document.ChannelName, strings.Join(trimNonEmptyConfirmationStrings(document.To), ", "), strings.Join(trimNonEmptyConfirmationStrings(document.People), ", ")))
	approvalQuestionSetDetail(details, "deliveryTargetType", document.TargetType)
	approvalQuestionSetDetail(details, "content", firstNonEmptyString(document.Message, document.Subject, document.Body, document.Title, document.Summary, document.ApprovalReason, document.Reason))
	approvalQuestionSetDetail(details, "message", document.Message)
	approvalQuestionSetDetail(details, "subject", document.Subject)
	approvalQuestionSetDetail(details, "body", document.Body)
	approvalQuestionSetDetail(details, "title", document.Title)
	approvalQuestionSetDetail(details, "summary", document.Summary)
	approvalQuestionSetDetail(details, "reason", document.Reason)
	approvalQuestionSetDetail(details, "approvalReason", document.ApprovalReason)
	approvalQuestionSetDetail(details, "slug", document.Slug)
	approvalQuestionSetDetail(details, "siteID", document.SiteID)
	approvalQuestionSetDetail(details, "eventHint", document.EventHint)
	filePath := firstNonEmptyString(document.Path, document.DevicePath, document.TargetPath)
	approvalQuestionSetDetail(details, "path", filePath)
	if strings.TrimSpace(filePath) != "" {
		approvalQuestionSetDetail(details, "fileName", filepath.Base(filePath))
	}
	if len(details) == 0 {
		return nil
	}
	return details
}

func approvalQuestionSetDetail(details map[string]string, key string, value string) {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return
	}
	details[key] = trimmedValue
}

func copyJSONRawMessage(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage{}, value...)
}
