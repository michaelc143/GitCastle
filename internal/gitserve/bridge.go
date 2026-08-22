package gitserve

import (
	"context"
	"errors"
)

// This file implements the Authorizer bridge against GitCastle's auth store.

// BridgeAuthorizer adapts concrete auth functions to the Authorizer
// interface, keeping this package free of imports from internal/auth.
type BridgeAuthorizer struct {
	AuthenticateFunc func(ctx context.Context, username, password string) (int64, error)
	CheckAccessFunc  func(ctx context.Context, userID int64, owner, repo string, access Access) error
}

func (b BridgeAuthorizer) Authenticate(ctx context.Context, username, password string) (int64, error) {
	return b.AuthenticateFunc(ctx, username, password)
}

func (b BridgeAuthorizer) CheckAccess(ctx context.Context, userID int64, owner, repo string, access Access) error {
	return b.CheckAccessFunc(ctx, userID, owner, repo, access)
}

var ErrAccessDenied = errors.New("access denied")
