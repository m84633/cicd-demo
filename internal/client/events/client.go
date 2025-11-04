package events

import (
	"context"
	"fmt"

	"github.com/arwoosa/event/gen/pb/common"
	pb "github.com/arwoosa/event/gen/pb/console"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a gRPC client for the event service.
type Client struct {
	conn   *grpc.ClientConn
	client pb.InternalServiceClient
}

// NewClient creates a new event service client.
func NewClient(addr string) (*Client, func(), error) {
	// Set up a connection to the server.
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to event service: %w", err)
	}

	cleanup := func() {
		conn.Close()
	}

	client := pb.NewInternalServiceClient(conn)

	return &Client{
		conn:   conn,
		client: client,
	}, cleanup, nil
}

// GetEventByID retrieves an event by its id.
func (c *Client) GetEventByID(ctx context.Context, eventID string) (*common.Event, error) {
	req := &common.ID{Id: eventID}
	return c.client.GetEventById(ctx, req)
}

func (c *Client) GetSessionByID(ctx context.Context, sessionID string) (*common.Session, error) {
	req := &common.ID{Id: sessionID}
	return c.client.GetSessionById(ctx, req)
}
