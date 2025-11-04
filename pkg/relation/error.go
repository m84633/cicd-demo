package relation

import (
	"errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrWriteConnectNotInitialed = errors.New("write connect not initialed")
	ErrReadConnectNotInitialed  = errors.New("read connect not initialed")
	ErrWriteFailed              = errors.New("write failed")
	ErrReadFailed               = errors.New("read failed")

	StatusRelationWriteConnectionFailed = status.New(codes.Aborted, "relation write connection failed")
	StatusRelationReadConnectionFailed  = status.New(codes.Aborted, "relation read connection failed")
	StatusRelationWriteFailed           = status.New(codes.Internal, "relation write failed")
	StatusRelationReadFailed            = status.New(codes.Internal, "relation read failed")
)

func ToStatus(err error) *status.Status {
	if err == nil {
		return nil
	}
	var baseSt *status.Status

	switch {
	case errors.Is(err, ErrWriteConnectNotInitialed):
		baseSt = StatusRelationWriteConnectionFailed
	case errors.Is(err, ErrReadConnectNotInitialed):
		baseSt = StatusRelationReadConnectionFailed
	case errors.Is(err, ErrWriteFailed):
		baseSt = StatusRelationWriteFailed
	case errors.Is(err, ErrReadFailed):
		baseSt = StatusRelationReadFailed
	default:
		// For unhandled errors, create a generic internal error status.
		return status.New(codes.Internal, err.Error())
	}
	unwrapErr := errors.Unwrap(err)
	if unwrapErr == nil {
		unwrapErr = err
	}
	// Add more details to the status, such as the type of violation and a description.
	st, myErr := baseSt.WithDetails(
		&errdetails.PreconditionFailure{
			Violations: []*errdetails.PreconditionFailure_Violation{
				{
					Type:        "RELATION",
					Subject:     unwrapErr.Error(),
					Description: err.Error(),
				},
			},
		},
	)
	if myErr != nil {
		// If adding details fails, return the original base status.
		return baseSt
	}
	return st
}
