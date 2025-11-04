package worker

import (
	"context"
	"partivo_tickets/internal/logic"
	"runtime/debug"
	"time"

	"go.uber.org/zap"
)

// TicketExpirer is a background worker that periodically expires overdue tickets.
type TicketExpirer struct {
	ticker *time.Ticker
	logic  *logic.TicketLogic
	logger *zap.Logger
	done   chan bool
}

// NewTicketExpirer creates a new TicketExpirer.
func NewTicketExpirer(interval time.Duration, logic *logic.TicketLogic, logger *zap.Logger) *TicketExpirer {
	return &TicketExpirer{
		ticker: time.NewTicker(interval),
		logic:  logic,
		logger: logger.Named("TicketExpirer"),
		done:   make(chan bool),
	}
}

// Start begins the ticker to periodically run the expiration task.
func (w *TicketExpirer) Start() {
	w.logger.Info("Starting ticket expirer worker")
	go func() {
		for {
			select {
			case <-w.done:
				return
			case <-w.ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							w.logger.Error("Panic recovered in ticket expirer worker",
								zap.Any("panic", r),
								zap.String("stack", string(debug.Stack())),
							)
						}
					}()
					w.logger.Debug("Running ticket expiration task")
					_, err := w.logic.ExpireOverdueTickets(context.Background())
					if err != nil {
						w.logger.Error("Failed to expire overdue tickets", zap.Error(err))
					}
				}()
			}
		}
	}()
}

// Stop stops the ticker.
func (w *TicketExpirer) Stop() {
	w.logger.Info("Stopping ticket expirer worker")
	w.ticker.Stop()
	w.done <- true
}
