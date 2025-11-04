package service

import (
	"encoding/json"
	"net/http"
	"partivo_tickets/api"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func ResponseError(c codes.Code, err error) (*api.Response, error) {
	return nil, status.Error(c, err.Error())
}

func ResponseSuccess(d any) (*api.Response, error) {
	var anyData *anypb.Any
	if d != nil {
		msg, ok := d.(proto.Message)
		if !ok {
			return nil, status.Error(codes.Internal, "output data is wrong")
		}

		var err error
		anyData, err = anypb.New(msg)
		if err != nil {
			return nil, status.Error(codes.Internal, "output data is wrong")
		}
	}

	return &api.Response{
		Status: "success",
		Code:   int32(CodeSuccess),
		//Message: CodeSuccess.Msg(),
		Data: anyData,
	}, nil
}

func ResponseSuccessWithMsg(d any, msg string) (*api.Response, error) {
	var anyData *anypb.Any
	if d != nil {
		p, ok := d.(proto.Message)
		if !ok {
			return nil, status.Error(codes.Internal, "output data is wrong")
		}

		var err error
		anyData, err = anypb.New(p)
		if err != nil {
			return nil, status.Error(codes.Internal, "output data is wrong")
		}
	}

	return &api.Response{
		Status:  "success",
		Code:    int32(CodeSuccess),
		Message: &msg,
		Data:    anyData,
	}, nil
}

// WriteHttpError writes a standard JSON error response to the http.ResponseWriter.
func WriteHttpError(w http.ResponseWriter, httpCode int, message string) {
	resp := map[string]interface{}{
		"status":  "error",
		"code":    httpCode,
		"message": message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(resp)
}
