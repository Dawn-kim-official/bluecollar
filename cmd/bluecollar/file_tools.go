package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/yeomyeonggeori/bluecollar/toolcontract"
)

type fileReadInput struct {
	Path string `json:"path"`
}

type fileWriteInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type fileEditInput struct {
	Path        string `json:"path"`
	FindText    string `json:"findText"`
	ReplaceText string `json:"replaceText"`
}

var fileReadInputSchema = json.RawMessage(`{
	"type": "object",
	"additionalProperties": false,
	"required": ["path"],
	"properties": {"path": {"type": "string", "description": "path to read, relative to the working directory"}}
}`)

var fileWriteInputSchema = json.RawMessage(`{
	"type": "object",
	"additionalProperties": false,
	"required": ["path", "content"],
	"properties": {
		"path": {"type": "string", "description": "path to write, relative to the working directory"},
		"content": {"type": "string", "description": "the complete new contents of the file"}
	}
}`)

var fileEditInputSchema = json.RawMessage(`{
	"type": "object",
	"additionalProperties": false,
	"required": ["path", "findText", "replaceText"],
	"properties": {
		"path": {"type": "string", "description": "path to edit, relative to the working directory"},
		"findText": {"type": "string", "description": "the exact text to replace, which must appear exactly once in the file"},
		"replaceText": {"type": "string", "description": "the text to put in its place"}
	}
}`)

var fileToolOutputSchema = json.RawMessage(`{
	"type": "object",
	"additionalProperties": false,
	"required": ["path", "content"],
	"properties": {
		"path": {"type": "string"},
		"content": {"type": "string"}
	}
}`)

func registerFileTools(toolSet *toolcontract.ToolSet, runningShell shell) {
	toolcontract.RegisterToolFunction(toolSet, toolcontract.ToolFunction[fileReadInput, toolcontract.ToolResult]{
		Definition: toolcontract.ToolDefinition{
			ID:              "bluecollar/file_read",
			Name:            toolcontract.FileReadToolName,
			SideEffectClass: toolcontract.ToolSideEffectRead,
			OutputSchema:    fileToolOutputSchema,
			ResultContract:  &toolcontract.ToolResultContract{Schema: fileToolOutputSchema},
			Description:     "Read a file and get back its exact contents.",
			Visibility:      toolcontract.ToolVisibilityModel,
			InputSchema:     fileReadInputSchema,
		},
		Result: toolcontract.IdentityToolResult,
		Handler: func(ctx context.Context, input fileReadInput) (toolcontract.ToolResult, error) {
			content, errorValue := runningShell.readFile(ctx, input.Path)
			if errorValue != nil {
				return toolcontract.ToolFailureResult(toolcontract.FailureNotFound, toolcontract.FailureCodes.NotFound,
					toolcontract.FileReadToolName, errorValue.Error()), nil
			}
			return toolcontract.ToolSuccessData(content, mustMarshalFileOutput(input.Path, content)), nil
		},
	})

	toolcontract.RegisterToolFunction(toolSet, toolcontract.ToolFunction[fileWriteInput, toolcontract.ToolResult]{
		Definition: toolcontract.ToolDefinition{
			ID:              "bluecollar/file_write",
			Name:            toolcontract.FileWriteToolName,
			SideEffectClass: toolcontract.ToolSideEffectStateChange,
			OutputSchema:    fileToolOutputSchema,
			ResultContract:  &toolcontract.ToolResultContract{Schema: fileToolOutputSchema},
			Description:     "Write a file from scratch, replacing whatever was there. To produce a modified version of a file that already exists, copy it and edit the copy; retyping its contents from memory changes lines you did not mean to change.",
			Visibility:      toolcontract.ToolVisibilityModel,
			InputSchema:     fileWriteInputSchema,
		},
		Result: toolcontract.IdentityToolResult,
		Handler: func(ctx context.Context, input fileWriteInput) (toolcontract.ToolResult, error) {
			if errorValue := runningShell.writeFile(ctx, input.Path, input.Content); errorValue != nil {
				return toolcontract.ToolFailureResult(toolcontract.FailureUnknown, toolcontract.FailureCodes.OperationFailed,
					toolcontract.FileWriteToolName, errorValue.Error()), nil
			}
			return fileChangeResult(input.Path, "wrote "+input.Path), nil
		},
	})

	toolcontract.RegisterToolFunction(toolSet, toolcontract.ToolFunction[fileEditInput, toolcontract.ToolResult]{
		Definition: toolcontract.ToolDefinition{
			ID:              "bluecollar/file_edit",
			Name:            toolcontract.FileEditToolName,
			SideEffectClass: toolcontract.ToolSideEffectStateChange,
			OutputSchema:    fileToolOutputSchema,
			ResultContract:  &toolcontract.ToolResultContract{Schema: fileToolOutputSchema},
			Description:     "Replace one exact passage of a file with another, leaving every other line byte for byte as it was. This is how an existing file should be changed.",
			Visibility:      toolcontract.ToolVisibilityModel,
			InputSchema:     fileEditInputSchema,
		},
		Result: toolcontract.IdentityToolResult,
		Handler: func(ctx context.Context, input fileEditInput) (toolcontract.ToolResult, error) {
			return editFileThroughShell(ctx, runningShell, input), nil
		},
	})
}

func editFileThroughShell(ctx context.Context, runningShell shell, input fileEditInput) toolcontract.ToolResult {
	content, errorValue := runningShell.readFile(ctx, input.Path)
	if errorValue != nil {
		return toolcontract.ToolFailureResult(toolcontract.FailureNotFound, toolcontract.FailureCodes.NotFound,
			toolcontract.FileEditToolName, errorValue.Error())
	}
	occurrences := strings.Count(content, input.FindText)
	if occurrences == 0 {
		return toolcontract.ToolFailureResult(toolcontract.FailureInvalidInput, toolcontract.FailureCodes.InvalidInput,
			toolcontract.FileEditToolName, "findText does not appear in "+input.Path+"; read the file and copy the passage exactly as it is written there")
	}
	if occurrences > 1 {
		return toolcontract.ToolFailureResult(toolcontract.FailureInvalidInput, toolcontract.FailureCodes.InvalidInput,
			toolcontract.FileEditToolName, "findText appears "+strconv.Itoa(occurrences)+" times in "+input.Path+"; include enough surrounding lines to name one passage")
	}
	if errorValue := runningShell.writeFile(ctx, input.Path, strings.Replace(content, input.FindText, input.ReplaceText, 1)); errorValue != nil {
		return toolcontract.ToolFailureResult(toolcontract.FailureUnknown, toolcontract.FailureCodes.OperationFailed,
			toolcontract.FileEditToolName, errorValue.Error())
	}
	return fileChangeResult(input.Path, "edited "+input.Path)
}

func fileChangeResult(path string, summary string) toolcontract.ToolResult {
	result := toolcontract.ToolSuccessData(summary, mustMarshalFileOutput(path, summary))
	result.Effects = []toolcontract.ResourceEffect{{ObjectType: "file", Effect: "changed", Path: path}}
	return result
}

func mustMarshalFileOutput(path string, content string) json.RawMessage {
	document, errorValue := json.Marshal(map[string]string{"path": path, "content": content})
	if errorValue != nil {
		return json.RawMessage(`{}`)
	}
	return document
}

func (runningShell shell) readFile(ctx context.Context, path string) (string, error) {
	capturedOutput := &bytes.Buffer{}
	command := runningShell.command(ctx, "base64 < "+shellQuoted(path))
	command.Stdout = capturedOutput
	if errorValue := command.Run(); errorValue != nil {
		return "", errors.New("could not read " + path)
	}
	decoded, errorValue := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(capturedOutput.String()), ""))
	if errorValue != nil {
		return "", errorValue
	}
	return string(decoded), nil
}

func (runningShell shell) writeFile(ctx context.Context, path string, content string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	command := runningShell.command(ctx, "printf %s "+shellQuoted(encoded)+" | base64 -d > "+shellQuoted(path))
	if errorValue := command.Run(); errorValue != nil {
		return errors.New("could not write " + path)
	}
	return nil
}

func shellQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
