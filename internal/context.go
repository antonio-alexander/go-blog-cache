package internal

import "context"

type ctxKeyCorrelationId struct{}

func CtxWithCorrelationId(ctx context.Context, correlationId string) context.Context {
	return context.WithValue(ctx, ctxKeyCorrelationId{}, correlationId)
}

func CorrelationIdFromCtx(ctx context.Context) string {
	item := ctx.Value(ctxKeyCorrelationId{})
	correlationId, ok := item.(string)
	if ok {
		return correlationId
	}
	return ""
}
