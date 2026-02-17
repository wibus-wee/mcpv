package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mcpv/internal/infra/daemon"
	controlv1 "mcpv/pkg/api/control/v1"
)

func writeJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func printToolsSnapshot(snapshot *controlv1.ToolsSnapshot, jsonOutput bool) error {
	if snapshot == nil {
		return nil
	}
	if jsonOutput {
		tools := make([]map[string]any, 0, len(snapshot.GetTools()))
		for _, tool := range snapshot.GetTools() {
			tools = append(tools, map[string]any{
				"name": tool.GetName(),
				"tool": json.RawMessage(tool.GetToolJson()),
			})
		}
		return writeJSON(map[string]any{
			"etag":  snapshot.GetEtag(),
			"tools": tools,
		})
	}
	fmt.Printf("etag=%s tools=%d\n", snapshot.GetEtag(), len(snapshot.GetTools()))
	for _, tool := range snapshot.GetTools() {
		fmt.Println(tool.GetName())
	}
	return nil
}

func printResourcesSnapshot(snapshot *controlv1.ResourcesSnapshot, nextCursor string, jsonOutput bool) error {
	if snapshot == nil {
		return nil
	}
	if jsonOutput {
		resources := make([]map[string]any, 0, len(snapshot.GetResources()))
		for _, res := range snapshot.GetResources() {
			resources = append(resources, map[string]any{
				"uri":      res.GetUri(),
				"resource": json.RawMessage(res.GetResourceJson()),
			})
		}
		payload := map[string]any{
			"etag":      snapshot.GetEtag(),
			"resources": resources,
		}
		if strings.TrimSpace(nextCursor) != "" {
			payload["nextCursor"] = nextCursor
		}
		return writeJSON(payload)
	}
	fmt.Printf("etag=%s resources=%d\n", snapshot.GetEtag(), len(snapshot.GetResources()))
	for _, res := range snapshot.GetResources() {
		fmt.Println(res.GetUri())
	}
	if strings.TrimSpace(nextCursor) != "" {
		fmt.Printf("nextCursor=%s\n", nextCursor)
	}
	return nil
}

func printPromptsSnapshot(snapshot *controlv1.PromptsSnapshot, nextCursor string, jsonOutput bool) error {
	if snapshot == nil {
		return nil
	}
	if jsonOutput {
		prompts := make([]map[string]any, 0, len(snapshot.GetPrompts()))
		for _, prompt := range snapshot.GetPrompts() {
			prompts = append(prompts, map[string]any{
				"name":   prompt.GetName(),
				"prompt": json.RawMessage(prompt.GetPromptJson()),
			})
		}
		payload := map[string]any{
			"etag":    snapshot.GetEtag(),
			"prompts": prompts,
		}
		if strings.TrimSpace(nextCursor) != "" {
			payload["nextCursor"] = nextCursor
		}
		return writeJSON(payload)
	}
	fmt.Printf("etag=%s prompts=%d\n", snapshot.GetEtag(), len(snapshot.GetPrompts()))
	for _, prompt := range snapshot.GetPrompts() {
		fmt.Println(prompt.GetName())
	}
	if strings.TrimSpace(nextCursor) != "" {
		fmt.Printf("nextCursor=%s\n", nextCursor)
	}
	return nil
}

func printResultPayload(label string, payload []byte, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(map[string]any{label: json.RawMessage(payload)})
	}
	fmt.Println(string(payload))
	return nil
}

func printTask(task *controlv1.Task, jsonOutput bool) error {
	if task == nil {
		return nil
	}
	if jsonOutput {
		return writeJSON(task)
	}
	fmt.Printf("task=%s status=%s\n", task.GetTaskId(), task.GetStatus())
	return nil
}

func printTasksList(resp *controlv1.TasksListResponse, jsonOutput bool) error {
	if resp == nil {
		return nil
	}
	if jsonOutput {
		return writeJSON(resp)
	}
	fmt.Printf("tasks=%d nextCursor=%s\n", len(resp.GetTasks()), resp.GetNextCursor())
	for _, task := range resp.GetTasks() {
		fmt.Printf("%s\t%s\n", task.GetTaskId(), task.GetStatus())
	}
	return nil
}

func printTaskResult(result *controlv1.TaskResult, jsonOutput bool) error {
	if result == nil {
		return nil
	}
	if jsonOutput {
		return writeJSON(map[string]any{
			"status":    result.GetStatus(),
			"result":    json.RawMessage(result.GetResultJson()),
			"errorCode": result.GetErrorCode(),
			"error": map[string]any{
				"message": result.GetErrorMessage(),
				"data":    json.RawMessage(result.GetErrorDataJson()),
			},
		})
	}
	fmt.Printf("status=%s\n", result.GetStatus())
	if data := result.GetResultJson(); len(data) > 0 {
		fmt.Println(string(data))
	}
	if result.GetErrorMessage() != "" {
		fmt.Printf("error=%s\n", result.GetErrorMessage())
	}
	return nil
}

func printLogEntry(entry *controlv1.LogEntry, jsonOutput bool) error {
	if entry == nil {
		return nil
	}
	if jsonOutput {
		return writeJSON(map[string]any{
			"logger":    entry.GetLogger(),
			"level":     entry.GetLevel().String(),
			"timestamp": entry.GetTimestampUnixNano(),
			"data":      json.RawMessage(entry.GetDataJson()),
		})
	}
	fmt.Printf("%s [%s] %s %s\n", time.Unix(0, entry.GetTimestampUnixNano()).Format(time.RFC3339Nano), entry.GetLevel().String(), entry.GetLogger(), string(entry.GetDataJson()))
	return nil
}

func printAutomaticMCP(resp *controlv1.AutomaticMCPResponse, jsonOutput bool) error {
	if resp == nil {
		return nil
	}
	if jsonOutput {
		tools := make([]json.RawMessage, 0, len(resp.GetToolsJson()))
		for _, raw := range resp.GetToolsJson() {
			tools = append(tools, json.RawMessage(raw))
		}
		return writeJSON(map[string]any{
			"etag":           resp.GetEtag(),
			"tools":          tools,
			"totalAvailable": resp.GetTotalAvailable(),
			"filtered":       resp.GetFiltered(),
		})
	}
	fmt.Printf("etag=%s total=%d filtered=%d\n", resp.GetEtag(), resp.GetTotalAvailable(), resp.GetFiltered())
	return nil
}

func printDaemonStatus(status daemon.Status, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(map[string]any{
			"installed":  status.Installed,
			"running":    status.Running,
			"service":    status.ServiceName,
			"configPath": status.ConfigPath,
			"rpcAddress": status.RPCAddress,
			"logPath":    status.LogPath,
		})
	}
	state := "stopped"
	if !status.Installed {
		state = "not installed"
	} else if status.Running {
		state = "running"
	}
	fmt.Printf("%s service=%s\n", state, status.ServiceName)
	if status.ConfigPath != "" {
		fmt.Printf("config=%s\n", status.ConfigPath)
	}
	if status.RPCAddress != "" {
		fmt.Printf("rpc=%s\n", status.RPCAddress)
	}
	if status.LogPath != "" {
		fmt.Printf("log=%s\n", status.LogPath)
	}
	return nil
}

func printDaemonAction(action string, status daemon.Status, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(map[string]any{
			"action":     action,
			"installed":  status.Installed,
			"running":    status.Running,
			"service":    status.ServiceName,
			"configPath": status.ConfigPath,
			"rpcAddress": status.RPCAddress,
			"logPath":    status.LogPath,
		})
	}
	fmt.Printf("%s service=%s\n", action, status.ServiceName)
	if status.ConfigPath != "" {
		fmt.Printf("config=%s\n", status.ConfigPath)
	}
	if status.RPCAddress != "" {
		fmt.Printf("rpc=%s\n", status.RPCAddress)
	}
	if status.LogPath != "" {
		fmt.Printf("log=%s\n", status.LogPath)
	}
	return nil
}
