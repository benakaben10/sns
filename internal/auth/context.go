package auth

import "context"

type contextKey string

const contextKeyUserInfo contextKey = "user_info"

type UserInfo struct {
	UserID   string
	Email    string
	Username string
	Claims   *Claims
}

func withUserInfo(ctx context.Context, info UserInfo) context.Context {
	return context.WithValue(ctx, contextKeyUserInfo, info)
}

// GetUserInfo retrieves authenticated user info from the request context.
func GetUserInfo(ctx context.Context) (UserInfo, bool) {
	info, ok := ctx.Value(contextKeyUserInfo).(UserInfo)
	return info, ok
}
