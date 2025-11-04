package worker

import "context"

// Worker defines the interface for a background process that can be started.
type Worker interface {
	Start(ctx context.Context)
}
