package cache

import (
	"context"
	"errors"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.elastic.co/apm/v2"
)

// apmHook records every Redis command as an Elastic APM span under the
// current transaction (Elastic has no official module for go-redis v9).
type apmHook struct{}

func (apmHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (apmHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		span, ctx := startRedisSpan(ctx, strings.ToUpper(cmd.Name()))
		defer span.End()
		err := next(ctx, cmd)
		markRedisOutcome(span, err)
		return err
	}
}

func (apmHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		span, ctx := startRedisSpan(ctx, "PIPELINE")
		defer span.End()
		err := next(ctx, cmds)
		markRedisOutcome(span, err)
		return err
	}
}

func startRedisSpan(ctx context.Context, name string) (*apm.Span, context.Context) {
	span, ctx := apm.StartSpan(ctx, name, "db.redis")
	span.Context.SetDatabase(apm.DatabaseSpanContext{Type: "redis", Statement: name})
	return span, ctx
}

// redis.Nil (key not found) is a normal cache miss, not a failure.
func markRedisOutcome(span *apm.Span, err error) {
	if err != nil && !errors.Is(err, redis.Nil) {
		span.Outcome = "failure"
	}
}
