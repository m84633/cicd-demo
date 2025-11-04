package relation

import (
	"context"
	"fmt"

	pb "github.com/ory/keto/proto/ory/keto/relation_tuples/v1alpha2"
)

func (c *Client) Check(ctx context.Context, namespace, object, relation string, subjectNamespace, subjectObject string) (bool, error) {
	if c.checkSC == nil {
		return false, ErrReadConnectNotInitialed
	}
	resp, err := c.checkSC.Check(ctx, &pb.CheckRequest{
		Tuple: &pb.RelationTuple{
			Namespace: namespace,
			Object:    object,
			Relation:  relation,
			Subject: &pb.Subject{
				Ref: &pb.Subject_Set{
					Set: &pb.SubjectSet{
						Namespace: subjectNamespace,
						Object:    subjectObject,
					},
				},
			},
		},
	})
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrReadFailed, err)
	}
	return resp.Allowed, nil
}

func (c *Client) CheckBySubjectId(ctx context.Context, namespace, object, relation string, subjectId string) (bool, error) {
	if c.checkSC == nil {
		return false, ErrReadConnectNotInitialed
	}

	resp, err := c.checkSC.Check(ctx, &pb.CheckRequest{
		Tuple: &pb.RelationTuple{
			Namespace: namespace,
			Object:    object,
			Relation:  relation,
			Subject: &pb.Subject{
				Ref: &pb.Subject_Id{
					Id: subjectId,
				},
			},
		},
	})
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrReadFailed, err)
	}
	return resp.Allowed, nil
}
