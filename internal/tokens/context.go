package tokens

import "context"

type key struct{}

var tokenKey key

func NewContext(ctx context.Context, token *Token) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

func FromContext(ctx context.Context) (*Token, bool) {
	d, ok := ctx.Value(tokenKey).(*Token)
	return d, ok
}
