package hooks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ReceiverConfig configures the hook receiver.
type ReceiverConfig struct {
	Port         int
	BindAddress  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	MaxBodySize  int64
}

// DefaultReceiverConfig returns default receiver configuration.
func DefaultReceiverConfig() ReceiverConfig {
	return ReceiverConfig{
		Port:         9876,
		BindAddress:  "127.0.0.1",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		MaxBodySize:  1 << 20, // 1MB
	}
}

// Receiver handles incoming hook events via HTTP.
type Receiver struct {
	config   ReceiverConfig
	server   *http.Server
	events   chan *ParsedEvent
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// NewReceiver creates a new hook receiver.
func NewReceiver(config ReceiverConfig) *Receiver {
	if config.Port == 0 {
		config.Port = 9876
	}
	if config.BindAddress == "" {
		config.BindAddress = "127.0.0.1"
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 5 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.MaxBodySize == 0 {
		config.MaxBodySize = 1 << 20
	}

	return &Receiver{
		config: config,
		events: make(chan *ParsedEvent, 1000), // Buffer up to 1000 events
	}
}

// Start begins listening for hook events.
func (r *Receiver) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", r.handleHook)
	mux.HandleFunc("/health", r.handleHealth)

	addr := fmt.Sprintf("%s:%d", r.config.BindAddress, r.config.Port)
	r.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  r.config.ReadTimeout,
		WriteTimeout: r.config.WriteTimeout,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Check for immediate startup errors
	select {
	case err := <-errCh:
		return fmt.Errorf("starting server: %w", err)
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
		return nil
	}
}

// Stop gracefully shuts down the receiver.
func (r *Receiver) Stop(ctx context.Context) error {
	var err error
	r.stopOnce.Do(func() {
		if r.server != nil {
			err = r.server.Shutdown(ctx)
		}
		close(r.events)
	})
	r.wg.Wait()
	return err
}

// Events returns the channel of parsed events.
func (r *Receiver) Events() <-chan *ParsedEvent {
	return r.events
}

// Address returns the receiver's listening address.
func (r *Receiver) Address() string {
	return fmt.Sprintf("%s:%d", r.config.BindAddress, r.config.Port)
}

func (r *Receiver) handleHook(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit body size
	req.Body = http.MaxBytesReader(w, req.Body, r.config.MaxBodySize)

	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Error reading body", http.StatusBadRequest)
		return
	}

	// Parse the event
	parsed, err := ParseEvent(body)
	if err != nil {
		http.Error(w, "Invalid event format", http.StatusBadRequest)
		return
	}

	// Validate event type
	if !IsValidEventType(parsed.Event.HookEventName) {
		http.Error(w, "Unknown event type", http.StatusBadRequest)
		return
	}

	// Send to channel (non-blocking)
	select {
	case r.events <- parsed:
		// Event queued
	default:
		// Channel full, drop event (could log this)
	}

	// Fast acknowledgment
	w.WriteHeader(http.StatusOK)
}

func (r *Receiver) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}
