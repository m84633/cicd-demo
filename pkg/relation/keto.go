// Package relation provides a client for Ory Keto, an open-source authorization server.
// It simplifies the process of creating and managing relation tuples for access control.
package relation

import (
	"context"
	"fmt"

	pb "github.com/ory/keto/proto/ory/keto/relation_tuples/v1alpha2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client holds the gRPC connections to the Keto APIs.
// It is safe for concurrent use.
type Client struct {
	writeConn *grpc.ClientConn
	readConn  *grpc.ClientConn
	writeSC   pb.WriteServiceClient
	readSC    pb.ReadServiceClient
	checkSC   pb.CheckServiceClient // Add Check Service Client
}

// Config holds the configuration for the Keto client.
type Config struct {
	WriteAddr string
	ReadAddr  string
}

// NewClient creates a new Keto client and its associated cleanup function.
func NewClient(cfg Config) (*Client, func(), error) {
	var writeConn, readConn *grpc.ClientConn
	var err error

	if cfg.WriteAddr != "" {
		writeConn, err = grpc.NewClient(cfg.WriteAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect to keto write api: %w", err)
		}
	}

	if cfg.ReadAddr != "" {
		readConn, err = grpc.NewClient(cfg.ReadAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			// Clean up write connection if read connection fails
			if writeConn != nil {
				writeConn.Close()
			}
			return nil, nil, fmt.Errorf("failed to connect to keto read api: %w", err)
		}
	}

	client := &Client{
		writeConn: writeConn,
		readConn:  readConn,
		writeSC:   pb.NewWriteServiceClient(writeConn),
		readSC:    pb.NewReadServiceClient(readConn),
		checkSC:   pb.NewCheckServiceClient(readConn), // Create the new client
	}

	cleanup := func() {
		if writeConn != nil {
			writeConn.Close()
		}
		if readConn != nil {
			readConn.Close()
		}
	}
	return client, cleanup, nil
}

// WriteTuple performs a transaction to create or delete relation tuples.
func (c *Client) WriteTuple(ctx context.Context, tuples tupleBuilder) error {
	if c.writeSC == nil {
		return ErrWriteConnectNotInitialed
	}

	_, err := c.writeSC.TransactRelationTuples(ctx, &pb.TransactRelationTuplesRequest{
		RelationTupleDeltas: tuples,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteFailed, err)
	}
	return nil
}
