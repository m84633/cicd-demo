package relation

import (
	"context"
	"fmt"

	pb "github.com/ory/keto/proto/ory/keto/relation_tuples/v1alpha2"
)

func (c *Client) DeleteObject(ctx context.Context, namespace, objectId string) error {
	if c.writeSC == nil {
		return ErrWriteConnectNotInitialed
	}
	_, err := c.writeSC.DeleteRelationTuples(ctx, &pb.DeleteRelationTuplesRequest{
		RelationQuery: &pb.RelationQuery{
			Namespace: &namespace,
			Object:    &objectId,
		},
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteFailed, err)
	}
	return nil
}
