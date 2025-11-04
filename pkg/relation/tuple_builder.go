package relation

import pb "github.com/ory/keto/proto/ory/keto/relation_tuples/v1alpha2"

type tupleBuilder []*pb.RelationTupleDelta

func NewTupleBuilder() tupleBuilder {
	return tupleBuilder{}
}

func (t *tupleBuilder) AppendInsertTupleWithSubjectId(namespace, object, relation, subject string) {
	*t = append(*t, &pb.RelationTupleDelta{
		Action: pb.RelationTupleDelta_ACTION_INSERT,
		RelationTuple: &pb.RelationTuple{
			Namespace: namespace,
			Object:    object,
			Relation:  relation,
			Subject: &pb.Subject{
				Ref: &pb.Subject_Id{
					Id: subject,
				},
			},
		},
	})
}

func (t *tupleBuilder) AppendInsertTupleWithSubjectSet(namespace, object, relation, subjectNamespace, subjectObject, subjectRelation string) {
	*t = append(*t, &pb.RelationTupleDelta{
		Action: pb.RelationTupleDelta_ACTION_INSERT,
		RelationTuple: &pb.RelationTuple{
			Namespace: namespace,
			Object:    object,
			Relation:  relation,
			Subject: &pb.Subject{
				Ref: &pb.Subject_Set{
					Set: &pb.SubjectSet{
						Namespace: subjectNamespace,
						Object:    subjectObject,
						Relation:  subjectRelation,
					},
				},
			},
		},
	})
}

func (t *tupleBuilder) AppendDeleteTupleWithSubjectId(namespace, object, relation, subject string) {
	*t = append(*t, &pb.RelationTupleDelta{
		Action: pb.RelationTupleDelta_ACTION_DELETE,
		RelationTuple: &pb.RelationTuple{
			Namespace: namespace,
			Object:    object,
			Relation:  relation,
			Subject: &pb.Subject{
				Ref: &pb.Subject_Id{
					Id: subject,
				},
			},
		},
	})
}

func (t *tupleBuilder) AppendDeleteTupleWithSubjectSet(namespace, object, relation, subjectNamespace, subjectObject string) {
	*t = append(*t, &pb.RelationTupleDelta{
		Action: pb.RelationTupleDelta_ACTION_DELETE,
		RelationTuple: &pb.RelationTuple{
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
}
