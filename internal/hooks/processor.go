package hooks

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/anthropics/mcp-lens/internal/storage"
)

// Processor handles event processing and storage.
type Processor struct {
	store      storage.Store
	identifier MCPIdentifier
	events     <-chan *ParsedEvent
	wg         sync.WaitGroup
	stopCh     chan struct{}

	// For latency calculation - track pending PreToolUse events
	pendingTools sync.Map // key: sessionID+toolName, value: time.Time
}

// NewProcessor creates a new event processor.
func NewProcessor(store storage.Store, events <-chan *ParsedEvent) *Processor {
	return &Processor{
		store:      store,
		identifier: NewRuleBasedIdentifier(),
		events:     events,
		stopCh:     make(chan struct{}),
	}
}

// Start begins processing events.
func (p *Processor) Start(ctx context.Context) {
	p.wg.Add(1)
	go p.processLoop(ctx)
}

// Stop gracefully stops the processor.
func (p *Processor) Stop() {
	close(p.stopCh)
	p.wg.Wait()
}

func (p *Processor) processLoop(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case event, ok := <-p.events:
			if !ok {
				return // Channel closed
			}
			if err := p.processEvent(ctx, event); err != nil {
				log.Printf("Error processing event: %v", err)
			}
		}
	}
}

func (p *Processor) processEvent(ctx context.Context, parsed *ParsedEvent) error {
	event := &storage.Event{
		SessionID:  parsed.Event.SessionID,
		EventType:  parsed.Event.HookEventName,
		RawPayload: parsed.Raw,
		CreatedAt:  parsed.ReceivedAt,
	}

	// Handle tool events
	if parsed.IsToolEvent() {
		event.ToolName = parsed.Tool.ToolName
		event.MCPServer = p.identifier.Identify(parsed.Tool.ToolName, parsed.Tool.ToolInput)
		event.Success = parsed.IsSuccess()

		// Calculate duration for PostToolUse events
		if parsed.Event.HookEventName == "PostToolUse" {
			key := parsed.Event.SessionID + ":" + parsed.Tool.ToolName
			if startTime, ok := p.pendingTools.LoadAndDelete(key); ok {
				event.DurationMs = time.Since(startTime.(time.Time)).Milliseconds()
			}
		} else if parsed.Event.HookEventName == "PreToolUse" {
			// Track start time for this tool call
			key := parsed.Event.SessionID + ":" + parsed.Tool.ToolName
			p.pendingTools.Store(key, parsed.ReceivedAt)
			// Don't store PreToolUse events to avoid duplicates
			// We only care about PostToolUse for metrics
			return nil
		}
	}

	return p.store.StoreEvent(ctx, event)
}

// ProcessorStats holds processor statistics.
type ProcessorStats struct {
	EventsProcessed int64
	EventsDropped   int64
	LastEventAt     time.Time
}

// SetIdentifier sets a custom MCP identifier.
func (p *Processor) SetIdentifier(identifier MCPIdentifier) {
	p.identifier = identifier
}
