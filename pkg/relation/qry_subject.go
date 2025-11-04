package relation

import (
	"context"
	"fmt"

	pb "github.com/ory/keto/proto/ory/keto/relation_tuples/v1alpha2"
)

type querySubjectResp struct {
	Namespace   string
	Object      string
	Relation    string
	SubjectIds  []string
	SubjectSets []struct {
		Namespace string
		Object    string
	}
}

func (o *querySubjectResp) AddSubjectId(id string) {
	o.SubjectIds = append(o.SubjectIds, id)
}

func (o *querySubjectResp) AddSubjectSet(namespace, object string) {
	o.SubjectSets = append(o.SubjectSets, struct {
		Namespace string
		Object    string
	}{
		Namespace: namespace,
		Object:    object,
	})
}

func (c *Client) QuerySubjectByObjectRelation(ctx context.Context, namespace, object, relation string) (*querySubjectResp, error) {
	if c.readSC == nil {
		return nil, ErrReadConnectNotInitialed
	}

	resp, err := c.readSC.ListRelationTuples(ctx, &pb.ListRelationTuplesRequest{
		Query: &pb.ListRelationTuplesRequest_Query{
			Namespace: namespace,
			Object:    object,
			Relation:  relation,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReadFailed, err)
	}

	result := &querySubjectResp{
		Namespace: namespace,
		Object:    object,
		Relation:  relation,
	}
	for _, rt := range resp.RelationTuples {
		if rt.Subject.GetId() != "" {
			result.AddSubjectId(rt.Subject.GetId())
		} else if rt.Subject.GetSet() != nil {
			result.AddSubjectSet(rt.Subject.GetSet().Namespace, rt.Subject.GetSet().Object)
		}
	}

	return result, nil
}
