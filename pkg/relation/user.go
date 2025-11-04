package relation

import (
	"context"
)

type role string

const (
	userNamespace = "User"
	RoleOwner     = role("owner")
	RoleEditor    = role("editor")
	RoleViewer    = role("viewer")
)

func (c *Client) AddUserResourceRole(ctx context.Context, userId, namespace, object string, setRole role) error {
	// check if user has role
	ok, err := c.Check(ctx, namespace, object, string(setRole), userNamespace, userId)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	// add user role
	tuples := NewTupleBuilder()
	tuples.AppendInsertTupleWithSubjectSet(namespace, object, string(setRole), userNamespace, userId, "")
	switch setRole {
	case RoleOwner:
		tuples.AppendInsertTupleWithSubjectSet(namespace, object, string(RoleEditor), namespace, object, string(RoleOwner))
		tuples.AppendInsertTupleWithSubjectSet(namespace, object, string(RoleViewer), namespace, object, string(RoleEditor))
	case RoleEditor:
		tuples.AppendInsertTupleWithSubjectSet(namespace, object, string(RoleViewer), namespace, object, string(RoleEditor))
	}
	return c.WriteTuple(ctx, tuples)
}
