package common

import "context"

func GetValueFromContext[T any](ctx context.Context, key string) (ret T, ok bool) {
	v := ctx.Value(key)
	if v == nil {
		return
	}
	if ret, ok = v.(T); ok {
		return ret, ok
	}
	return
}
