package telemetry

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"

	"mcpd/internal/domain"
)

type LogBroadcaster struct {
	minLevel zapcore.Level
	mu       sync.RWMutex
	subs     map[chan domain.LogEntry]struct{}
}

func NewLogBroadcaster(minLevel zapcore.Level) *LogBroadcaster {
	return &LogBroadcaster{
		minLevel: minLevel,
		subs:     make(map[chan domain.LogEntry]struct{}),
	}
}

func (b *LogBroadcaster) Core() zapcore.Core {
	return &logBroadcasterCore{broadcaster: b}
}

func (b *LogBroadcaster) Subscribe(ctx context.Context) <-chan domain.LogEntry {
	ch := make(chan domain.LogEntry, DefaultLogBufferSize)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subs, ch)
		close(ch)
		b.mu.Unlock()
	}()

	return ch
}

func (b *LogBroadcaster) publish(entry domain.LogEntry) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- entry:
		default:
		}
	}
}

type logBroadcasterCore struct {
	broadcaster *LogBroadcaster
	fields      []zapcore.Field
}

func (c *logBroadcasterCore) Enabled(level zapcore.Level) bool {
	return level >= c.broadcaster.minLevel
}

func (c *logBroadcasterCore) With(fields []zapcore.Field) zapcore.Core {
	if len(fields) == 0 {
		return c
	}
	combined := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	combined = append(combined, c.fields...)
	combined = append(combined, fields...)
	return &logBroadcasterCore{broadcaster: c.broadcaster, fields: combined}
}

func (c *logBroadcasterCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *logBroadcasterCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	dataJSON, err := c.buildDataJSON(entry, fields)
	if err != nil {
		return nil
	}

	c.broadcaster.publish(domain.LogEntry{
		Logger:    c.loggerName(entry.LoggerName),
		Level:     mapZapLevel(entry.Level),
		Timestamp: entry.Time,
		DataJSON:  dataJSON,
	})
	return nil
}

func (c *logBroadcasterCore) Sync() error {
	return nil
}

func (c *logBroadcasterCore) loggerName(entryName string) string {
	if entryName != "" {
		return entryName
	}
	return "mcpd"
}

func (c *logBroadcasterCore) buildDataJSON(entry zapcore.Entry, fields []zapcore.Field) (json.RawMessage, error) {
	encoder := zapcore.NewMapObjectEncoder()
	for _, field := range c.fields {
		field.AddTo(encoder)
	}
	for _, field := range fields {
		field.AddTo(encoder)
	}

	data := map[string]any{
		"message":   entry.Message,
		"timestamp": entry.Time.UTC().Format(time.RFC3339Nano),
	}
	if entry.LoggerName != "" {
		data["logger"] = entry.LoggerName
	}
	if len(encoder.Fields) > 0 {
		data["fields"] = encoder.Fields
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func mapZapLevel(level zapcore.Level) domain.LogLevel {
	switch level {
	case zapcore.DebugLevel:
		return domain.LogLevelDebug
	case zapcore.InfoLevel:
		return domain.LogLevelInfo
	case zapcore.WarnLevel:
		return domain.LogLevelWarning
	case zapcore.ErrorLevel:
		return domain.LogLevelError
	case zapcore.DPanicLevel:
		return domain.LogLevelCritical
	case zapcore.PanicLevel:
		return domain.LogLevelAlert
	case zapcore.FatalLevel:
		return domain.LogLevelEmergency
	default:
		return domain.LogLevelInfo
	}
}

var _ zapcore.Core = (*logBroadcasterCore)(nil)
