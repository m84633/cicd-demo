package service

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	userID     = "X-User-Id"
	userEmail  = "X-User-Email"
	userName   = "X-User-Name"
	userAvatar = "X-User-Avatar"

	DefaultRequestTimeout = 5 * time.Second
)

func getUserId(c context.Context) (userId primitive.ObjectID, err error) {
	md, ok := metadata.FromIncomingContext(c)
	if ok == false {
		return primitive.ObjectID{}, status.Errorf(codes.Unauthenticated, "unauthenticated")
	}

	d := md.Get(userID)
	if len(d) == 0 {
		return primitive.ObjectID{}, status.Errorf(codes.Unauthenticated, "unauthenticated")
	}
	u := d[0]

	uid, err := primitive.ObjectIDFromHex(u)
	if err != nil {
		return primitive.ObjectID{}, status.Errorf(codes.Unauthenticated, "unauthenticated")
	}

	return uid, nil
}

func getUserEmail(c context.Context) (email string) {
	md, ok := metadata.FromIncomingContext(c)
	if ok == false {
		return
	}
	d := md.Get(userEmail)
	if len(d) == 0 {
		return
	}
	email = d[0]
	return
}

func getUserName(c context.Context) (name string) {
	md, ok := metadata.FromIncomingContext(c)
	if ok == false {
		return
	}
	d := md.Get(userName)
	if len(d) == 0 {
		return
	}
	name = d[0]
	return
}

func getUserAvatar(c context.Context) (avatar string) {
	md, ok := metadata.FromIncomingContext(c)
	if ok == false {
		return
	}
	d := md.Get(userAvatar)
	if len(d) == 0 {
		return
	}
	avatar = d[0]
	return
}
