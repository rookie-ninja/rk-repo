package rkasynq

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/rookie-ninja/rk-entry/v2/middleware/tracing"
	"go.opentelemetry.io/otel/codes"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
	"net/http"
)

var (
	noopTracerProvider = trace.NewNoopTracerProvider()
)

const (
	spanKey      = "SpanKey"
	traceIdKey   = "TraceIdKey"
	optionSetKey = "OptionSet"
)

type basePayload struct {
	TraceHeader http.Header `json:"traceHeader"`
}

type TraceConfig struct {
	Asynq struct {
		Trace rkmidtrace.BootConfig `yaml:"trace"`
	} `yaml:"asynq"`
}

func NewJaegerMid(traceRaw []byte) (asynq.MiddlewareFunc, error) {
	conf := &TraceConfig{}
	err := yaml.Unmarshal(traceRaw, conf)

	if err != nil {
		return nil, err
	}

	mid := &TraceMiddleware{
		set: rkmidtrace.NewOptionSet(
			rkmidtrace.ToOptions(&conf.Asynq.Trace, "worker", "ansynq")...),
	}

	return mid.Middleware, nil
}

type TraceMiddleware struct {
	set rkmidtrace.OptionSetInterface
}

func (m *TraceMiddleware) Middleware(h asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		var p basePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
		}

		ctx = m.set.GetPropagator().Extract(ctx, propagation.HeaderCarrier(p.TraceHeader))
		spanCtx := trace.SpanContextFromContext(ctx)

		// create new span
		ctx, span := m.set.GetTracer().Start(trace.ContextWithRemoteSpanContext(ctx, spanCtx), t.Type())
		defer span.End()

		ctx = context.WithValue(ctx, spanKey, span)
		ctx = context.WithValue(ctx, traceIdKey, span.SpanContext().TraceID())
		ctx = context.WithValue(ctx, optionSetKey, m.set)

		err := h.ProcessTask(ctx, t)

		if err != nil {
			span.SetStatus(codes.Error, fmt.Sprintf("%v", err))
		} else {
			span.SetStatus(codes.Ok, "success")
		}

		return err
	})
}

func GetSpan(ctx context.Context) trace.Span {
	if v := ctx.Value(spanKey); v != nil {
		if res, ok := v.(trace.Span); ok {
			return res
		}
	}

	_, span := noopTracerProvider.Tracer("rk-trace-noop").Start(ctx, "noop-span")
	return span
}

func GetTraceId(ctx context.Context) string {
	return GetSpan(ctx).SpanContext().TraceID().String()
}

func GetTracer(ctx context.Context) trace.Tracer {
	if v := ctx.Value(optionSetKey); v != nil {
		if res, ok := v.(rkmidtrace.OptionSetInterface); ok {
			return res.GetTracer()
		}
	}

	return noopTracerProvider.Tracer("rk-trace-noop")
}

func GetPropagator(ctx context.Context) propagation.TextMapPropagator {
	if v := ctx.Value(optionSetKey); v != nil {
		if res, ok := v.(rkmidtrace.OptionSetInterface); ok {
			return res.GetPropagator()
		}
	}

	return nil
}

func GetProvider(ctx context.Context) *sdktrace.TracerProvider {
	if v := ctx.Value(optionSetKey); v != nil {
		if res, ok := v.(rkmidtrace.OptionSetInterface); ok {
			return res.GetProvider()
		}
	}

	return nil
}

func NewSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return GetTracer(ctx).Start(ctx, name)
}

func EndSpan(span trace.Span, success bool) {
	if success {
		span.SetStatus(otelcodes.Ok, otelcodes.Ok.String())
	}

	span.End()
}
