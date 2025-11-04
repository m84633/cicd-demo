package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// SystemUser represents an operator for actions performed by the system itself.
var SystemUser = &User{
	UserId: primitive.NilObjectID, // Or a specific, known ID for the system user
	Name:   "System",
	Email:  "system@partivo.com",
}

type User struct {
	UserId primitive.ObjectID `json:"user_id" bson:"user_id"`
	Name   string             `json:"name" bson:"name"`
	Email  string             `json:"email" bson:"email"`
	Avatar string             `json:"avatar" bson:"avatar"`
}
