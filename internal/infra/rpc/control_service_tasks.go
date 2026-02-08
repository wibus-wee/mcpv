package rpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (s *ControlService) TasksGet(ctx context.Context, req *controlv1.TasksGetRequest) (*controlv1.TasksGetResponse, error) {
	if req.GetTaskId() == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}
	task, err := s.control.GetTask(ctx, req.GetCaller(), req.GetTaskId())
	if err != nil {
		return nil, statusFromError("get task", err)
	}
	return &controlv1.TasksGetResponse{Task: toProtoTask(task)}, nil
}

func (s *ControlService) TasksList(ctx context.Context, req *controlv1.TasksListRequest) (*controlv1.TasksListResponse, error) {
	page, err := s.control.ListTasks(ctx, req.GetCaller(), req.GetCursor(), int(req.GetLimit()))
	if err != nil {
		return nil, statusFromError("list tasks", err)
	}
	tasks := make([]*controlv1.Task, 0, len(page.Tasks))
	for _, task := range page.Tasks {
		tasks = append(tasks, toProtoTask(task))
	}
	return &controlv1.TasksListResponse{
		Tasks:      tasks,
		NextCursor: page.NextCursor,
	}, nil
}

func (s *ControlService) TasksResult(ctx context.Context, req *controlv1.TasksResultRequest) (*controlv1.TasksResultResponse, error) {
	if req.GetTaskId() == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}
	result, err := s.control.GetTaskResult(ctx, req.GetCaller(), req.GetTaskId())
	if err != nil {
		return nil, statusFromError("get task result", err)
	}
	return &controlv1.TasksResultResponse{
		Result: toProtoTaskResult(result),
	}, nil
}

func (s *ControlService) TasksCancel(ctx context.Context, req *controlv1.TasksCancelRequest) (*controlv1.TasksCancelResponse, error) {
	if req.GetTaskId() == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}
	task, err := s.control.CancelTask(ctx, req.GetCaller(), req.GetTaskId())
	if err != nil {
		return nil, statusFromError("cancel task", err)
	}
	return &controlv1.TasksCancelResponse{Task: toProtoTask(task)}, nil
}

func toProtoTask(task domain.Task) *controlv1.Task {
	if task.TaskID == "" {
		return &controlv1.Task{}
	}
	ttl := int64(0)
	if task.TTL != nil {
		ttl = *task.TTL
	}
	poll := int64(0)
	if task.PollInterval != nil {
		poll = *task.PollInterval
	}
	return &controlv1.Task{
		TaskId:         task.TaskID,
		Status:         string(task.Status),
		StatusMessage:  task.StatusMessage,
		CreatedAt:      task.CreatedAt.UTC().Format(time.RFC3339Nano),
		LastUpdatedAt:  task.LastUpdatedAt.UTC().Format(time.RFC3339Nano),
		TtlMs:          ttl,
		PollIntervalMs: poll,
	}
}

func toProtoTaskResult(result domain.TaskResult) *controlv1.TaskResult {
	resp := &controlv1.TaskResult{
		Status: string(result.Status),
	}
	if len(result.Result) > 0 {
		resp.ResultJson = result.Result
	}
	if result.Error != nil {
		resp.ErrorCode = result.Error.Code
		resp.ErrorMessage = result.Error.Message
		resp.ErrorDataJson = result.Error.Data
	}
	return resp
}
