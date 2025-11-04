package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrInvalidToken = errors.New("invalid or expired token")

type PageToken string

// Page represents the cursor data for pagination.
type Page struct {
	CursorID        string `json:"id"`
	CursorTimestamp int64  `json:"ts"`
}

// Encode converts the Page struct into a base64-encoded token.
func (p Page) Encode() (PageToken, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return PageToken(base64.StdEncoding.EncodeToString(b)), nil
}

// Decode converts a base64-encoded token back into a Page struct.
func (t PageToken) Decode() (*Page, error) {
	var p Page
	if len(t) == 0 {
		return nil, nil // No token provided, indicates first page
	}

	bytes, err := base64.StdEncoding.DecodeString(string(t))
	if err != nil {
		return nil, ErrInvalidToken
	}

	err = json.Unmarshal(bytes, &p)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Optional: Check if the token is expired (e.g., > 24 hours old)
	if time.Unix(p.CursorTimestamp, 0).Before(time.Now().Add(-24 * time.Hour)) {
		return nil, ErrInvalidToken
	}

	return &p, nil
}

// GenerateToken creates a new PageToken from a cursor's id and timestamp.
func GenerateToken(id primitive.ObjectID, ts time.Time) (PageToken, error) {
	p := Page{
		CursorID:        id.Hex(),
		CursorTimestamp: ts.Unix(),
	}
	return p.Encode()
}
