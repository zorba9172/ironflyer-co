// Package tracing centralises OpenTelemetry wiring for the orchestrator.
//
// Init installs a global TracerProvider matching the requested exporter.
// The package degrades gracefully: an unknown / empty / "none" exporter
// installs a no-op provider so callers can always rely on Tracer() and
// StartSpan() without nil checks. A failure to construct an OTLP/stdout
// exporter is returned from Init so main.go can log it and continue with
// the no-op fallback already installed via otel.SetTracerProvider.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	noopprovider "go.opentelemetry.io/otel/trace/noop"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Opts configures Init. See package comment for the per-exporter behaviour.
type Opts struct {
	Exporter       string            // "none" | "stdout" | "otlp"
	Endpoint       string            // for otlp, e.g. "https://api.honeycomb.io:443"
	Insecure       bool              // for otlp over plaintext
	ServiceName    string            // default "ironflyer-orchestrator"
	ServiceVersion string            // service.version resource attribute
	Headers        map[string]string // sent on otlp requests (auth header, etc.)
	SampleRatio    float64           // 0..1; 1.0 = always sample, 0 = never. Default 1.0 in dev.
}

const tracerName = "ironflyer"

// noopShutdown is the shutdown func returned when we install a no-op
// TracerProvider. Keeping it explicit means callers can always invoke
// the returned shutdown without a nil guard.
func noopShutdown(context.Context) error { return nil }

// Init installs a global TracerProvider per Opts. The returned shutdown
// MUST be called on orchestrator exit to flush pending spans.
func Init(ctx context.Context, opts Opts) (func(context.Context) error, error) {
	if opts.ServiceName == "" {
		opts.ServiceName = "ironflyer-orchestrator"
	}
	if opts.SampleRatio < 0 {
		opts.SampleRatio = 0
	}
	if opts.SampleRatio > 1 {
		opts.SampleRatio = 1
	}

	switch opts.Exporter {
	case "", "none":
		// Install a real no-op provider so otel.Tracer() never returns nil.
		otel.SetTracerProvider(noopprovider.NewTracerProvider())
		return noopShutdown, nil
	case "stdout", "otlp":
		// handled below
	default:
		otel.SetTracerProvider(noopprovider.NewTracerProvider())
		return noopShutdown, fmt.Errorf("tracing: unknown exporter %q", opts.Exporter)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(opts.ServiceName),
			semconv.ServiceVersion(opts.ServiceVersion),
		),
	)
	if err != nil {
		// Fall back to the default resource — never block startup on a
		// resource merge failure.
		res = resource.Default()
	}

	var exporter sdktrace.SpanExporter
	switch opts.Exporter {
	case "stdout":
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			otel.SetTracerProvider(noopprovider.NewTracerProvider())
			return noopShutdown, fmt.Errorf("tracing: stdout exporter: %w", err)
		}
		exporter = exp
	case "otlp":
		httpOpts := []otlptracehttp.Option{}
		if opts.Endpoint != "" {
			httpOpts = append(httpOpts, otlptracehttp.WithEndpoint(opts.Endpoint))
		}
		if opts.Insecure {
			httpOpts = append(httpOpts, otlptracehttp.WithInsecure())
		}
		if len(opts.Headers) > 0 {
			httpOpts = append(httpOpts, otlptracehttp.WithHeaders(opts.Headers))
		}
		exp, err := otlptrace.New(ctx, otlptracehttp.NewClient(httpOpts...))
		if err != nil {
			otel.SetTracerProvider(noopprovider.NewTracerProvider())
			return noopShutdown, fmt.Errorf("tracing: otlp exporter: %w", err)
		}
		exporter = exp
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(opts.SampleRatio))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// Tracer returns the global ironflyer tracer. Safe to call before Init —
// it falls back to the otel global, which is a no-op until Init runs.
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// StartSpan is the package convenience: equivalent to
// tracing.Tracer().Start(ctx, name, trace.WithAttributes(attrs...)).
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}
