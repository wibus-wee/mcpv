package pipeline

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
)

var categoryOrder = []domain.PluginCategory{
	domain.PluginCategoryObservability,
	domain.PluginCategoryAuthentication,
	domain.PluginCategoryAuthorization,
	domain.PluginCategoryRateLimiting,
	domain.PluginCategoryValidation,
	domain.PluginCategoryContent,
	domain.PluginCategoryAudit,
}

type Engine struct {
	handler Handler
	logger  *zap.Logger
	metrics domain.Metrics

	mu         sync.RWMutex
	plugins    []domain.PluginSpec
	byCategory map[domain.PluginCategory][]domain.PluginSpec
}

type Handler interface {
	Handle(ctx context.Context, spec domain.PluginSpec, req domain.GovernanceRequest) (domain.GovernanceDecision, error)
}

func NewEngine(handler Handler, logger *zap.Logger, metrics domain.Metrics) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Engine{
		handler:    handler,
		logger:     logger.Named("pipeline"),
		metrics:    metrics,
		plugins:    nil,
		byCategory: make(map[domain.PluginCategory][]domain.PluginSpec),
	}
}

func (e *Engine) Update(specs []domain.PluginSpec) {
	byCategory := make(map[domain.PluginCategory][]domain.PluginSpec)
	copied := append([]domain.PluginSpec(nil), specs...)
	for _, spec := range copied {
		byCategory[spec.Category] = append(byCategory[spec.Category], spec)
	}
	for _, list := range byCategory {
		sort.Slice(list, func(i, j int) bool {
			return list[i].Name < list[j].Name
		})
	}

	e.mu.Lock()
	e.plugins = copied
	e.byCategory = byCategory
	e.mu.Unlock()
}

func (e *Engine) Handle(ctx context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
	if req.Flow == "" {
		req.Flow = domain.PluginFlowRequest
	}

	e.mu.RLock()
	byCategory := e.byCategory
	e.mu.RUnlock()

	decision := domain.GovernanceDecision{Continue: true}
	request := req

	for _, category := range categoryOrder {
		plugins := byCategory[category]
		if len(plugins) == 0 {
			continue
		}
		if category == domain.PluginCategoryObservability {
			if err := e.runObservability(ctx, plugins, request, request.Flow); err != nil {
				return decision, err
			}
			continue
		}
		var err error
		request, decision, err = e.runSequential(ctx, category, plugins, request, request.Flow)
		if err != nil {
			return decision, err
		}
		if !decision.Continue {
			return decision, nil
		}
	}

	return decision, nil
}

func (e *Engine) runObservability(ctx context.Context, plugins []domain.PluginSpec, req domain.GovernanceRequest, flow domain.PluginFlow) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(plugins))

	for _, spec := range plugins {
		if !flowAllowed(spec, flow) {
			continue
		}
		wg.Add(1)
		spec := spec
		go func() {
			defer wg.Done()
			start := time.Now()
			decision, err := e.handler.Handle(ctx, spec, req)
			duration := time.Since(start)
			if err != nil {
				e.recordOutcome(spec, flow, domain.GovernanceOutcomePluginError, duration)
				if spec.Required {
					errCh <- err
					return
				}
				e.logger.Debug("observability plugin error ignored", zap.String("plugin", spec.Name), zap.Error(err))
				return
			}
			if !decision.Continue {
				code := defaultRejectCode(decision.RejectCode, spec.Category)
				decision.RejectCode = code
				decision.RejectMessage = defaultRejectMessage(decision.RejectMessage)
				e.recordOutcome(spec, flow, domain.GovernanceOutcomeRejected, duration)
				e.recordRejection(spec, flow, code)
				if spec.Required {
					errCh <- domain.GovernanceRejection{
						Category: spec.Category,
						Plugin:   spec.Name,
						Code:     decision.RejectCode,
						Message:  decision.RejectMessage,
					}
				}
				return
			}
			e.recordOutcome(spec, flow, domain.GovernanceOutcomeContinue, duration)
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) runSequential(ctx context.Context, category domain.PluginCategory, plugins []domain.PluginSpec, req domain.GovernanceRequest, flow domain.PluginFlow) (domain.GovernanceRequest, domain.GovernanceDecision, error) {
	current := req
	decision := domain.GovernanceDecision{Continue: true}

	for _, spec := range plugins {
		if !flowAllowed(spec, flow) {
			continue
		}
		start := time.Now()
		resp, err := e.handler.Handle(ctx, spec, current)
		duration := time.Since(start)
		if err != nil {
			e.recordOutcome(spec, flow, domain.GovernanceOutcomePluginError, duration)
			if spec.Required {
				return current, decision, err
			}
			e.logger.Debug("optional plugin error ignored", zap.String("plugin", spec.Name), zap.Error(err))
			continue
		}
		if !resp.Continue {
			e.recordOutcome(spec, flow, domain.GovernanceOutcomeRejected, duration)
			code := defaultRejectCode(resp.RejectCode, category)
			resp.RejectCode = code
			resp.RejectMessage = defaultRejectMessage(resp.RejectMessage)
			e.recordRejection(spec, flow, code)
			if !spec.Required && shouldIgnoreOptionalRejection(category) {
				e.logger.Debug("optional rejection ignored", zap.String("plugin", spec.Name))
				continue
			}
			decision = resp
			decision.Continue = false
			decision.Category = category
			decision.Plugin = spec.Name
			return current, decision, nil
		}
		e.recordOutcome(spec, flow, domain.GovernanceOutcomeContinue, duration)

		if category == domain.PluginCategoryContent {
			current = applyMutations(current, resp)
		} else if len(resp.RequestJSON) > 0 || len(resp.ResponseJSON) > 0 {
			e.logger.Warn("non-content plugin returned mutations", zap.String("plugin", spec.Name), zap.String("category", string(category)))
		}
	}

	return current, decision, nil
}

func flowAllowed(spec domain.PluginSpec, flow domain.PluginFlow) bool {
	if len(spec.Flows) == 0 {
		return true
	}
	for _, f := range spec.Flows {
		if f == flow {
			return true
		}
	}
	return false
}

func applyMutations(req domain.GovernanceRequest, decision domain.GovernanceDecision) domain.GovernanceRequest {
	updated := req
	if len(decision.RequestJSON) > 0 {
		updated.RequestJSON = decision.RequestJSON
	}
	if len(decision.ResponseJSON) > 0 {
		updated.ResponseJSON = decision.ResponseJSON
	}
	return updated
}

func shouldIgnoreOptionalRejection(category domain.PluginCategory) bool {
	return category == domain.PluginCategoryObservability
}

func defaultRejectCode(code string, category domain.PluginCategory) string {
	if code != "" {
		return code
	}
	switch category {
	case domain.PluginCategoryAuthentication:
		return "unauthenticated"
	case domain.PluginCategoryAuthorization:
		return "unauthorized"
	case domain.PluginCategoryRateLimiting:
		return "rate_limited"
	case domain.PluginCategoryValidation:
		return "invalid_request"
	case domain.PluginCategoryObservability,
		domain.PluginCategoryContent,
		domain.PluginCategoryAudit:
		return "rejected"
	default:
		return "rejected"
	}
}

func defaultRejectMessage(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return "request rejected"
	}
	return msg
}

func (e *Engine) recordOutcome(spec domain.PluginSpec, flow domain.PluginFlow, outcome domain.GovernanceOutcome, duration time.Duration) {
	if e.metrics == nil {
		return
	}
	if duration < 0 {
		duration = 0
	}
	e.metrics.RecordGovernanceOutcome(domain.GovernanceOutcomeMetric{
		Category: spec.Category,
		Plugin:   spec.Name,
		Flow:     flow,
		Outcome:  outcome,
		Duration: duration,
	})
}

func (e *Engine) recordRejection(spec domain.PluginSpec, flow domain.PluginFlow, code string) {
	if e.metrics == nil {
		return
	}
	e.metrics.RecordGovernanceRejection(domain.GovernanceRejectionMetric{
		Category: spec.Category,
		Plugin:   spec.Name,
		Flow:     flow,
		Code:     code,
	})
}
