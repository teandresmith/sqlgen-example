// Package observability wires OpenTelemetry metrics for the task-tracker
// server (PRD story 56). It owns two concerns the rest of the app should not
// have to know about:
//
//   - Setup builds an OTLP-exporting MeterProvider from the configured
//     collector endpoint and installs it as the process-global provider, so
//     any instrument created with the global default (including the generated
//     cache's OTel MetricsRecorder) exports to the collector.
//   - Middleware instruments the HTTP surface with request metrics, and
//     CacheRecorder adapts the sqlgen OTel recorder for the generated cache —
//     the "attached to the client and the HTTP server" halves of story 56.
//
// Metric export is optional: when the endpoint is empty, Setup returns a
// no-op provider so the server runs identically without a collector (config
// documents OTEL_EXPORTER_OTLP_ENDPOINT as optional). Nothing in the request
// path changes based on whether export is on — the same instruments are
// recorded either way; a no-op provider simply drops the readings.
package observability

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

// serviceName labels every exported metric with the emitting service so a
// shared collector can attribute readings to this server.
const serviceName = "taskr-server"

// Provider owns the configured MeterProvider and the shutdown hook that
// flushes and releases its exporter. Get the MeterProvider to build
// instruments; call Shutdown once during graceful shutdown so the final
// batch of metrics is flushed before the process exits.
type Provider struct {
	mp       metric.MeterProvider
	shutdown func(context.Context) error
}

// MeterProvider returns the configured provider. It is never nil — a
// Provider from Setup with an empty endpoint returns a no-op provider — so
// callers can build instruments unconditionally.
func (p *Provider) MeterProvider() metric.MeterProvider { return p.mp }

// Shutdown flushes any buffered metrics and releases the exporter. It is
// safe to call on a no-op Provider (it does nothing) and safe to call with
// an already-cancelled context (the SDK still attempts a best-effort flush).
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.shutdown == nil {
		return nil
	}
	return p.shutdown(ctx)
}

// Setup builds a MeterProvider that exports metrics to the OTLP collector at
// endpoint. The server's own instruments receive this provider explicitly (the
// cache recorder via CacheRecorder, the HTTP middleware via MeterProvider); it
// is additionally installed as the process-global provider (otel.SetMeterProvider)
// so any transitively OTel-instrumented dependency, and the conventional
// otel.GetMeterProvider() default, resolve to the same collector rather than a
// no-op. When endpoint is empty, metric export is disabled: a no-op provider is
// returned and the global provider is left untouched.
//
// The endpoint is the OTLP gRPC target (OTEL_EXPORTER_OTLP_ENDPOINT), e.g.
// "localhost:4317" or "http://otel-collector:4317". A scheme, if present, is
// stripped; "https://" selects TLS, anything else (or no scheme) is treated
// as an insecure local link, which matches the docker-compose collector.
func Setup(ctx context.Context, endpoint string) (*Provider, error) {
	if strings.TrimSpace(endpoint) == "" {
		return &Provider{mp: metricnoop.NewMeterProvider()}, nil
	}

	target, insecure := parseEndpoint(endpoint)
	opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(target)}
	if insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("observability: creating OTLP metric exporter: %w", err)
	}

	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	))
	if err != nil {
		// A schema-URL mismatch between Default and our attributes is the only
		// way this fails; fall back to just our attributes rather than aborting
		// boot over a resource-labeling detail.
		res = resource.NewSchemaless(semconv.ServiceName(serviceName))
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)
	otel.SetMeterProvider(mp)

	return &Provider{mp: mp, shutdown: mp.Shutdown}, nil
}

// parseEndpoint splits an OTLP endpoint into the host:port target the gRPC
// exporter wants and whether the link is insecure. otlpmetricgrpc.WithEndpoint
// rejects a scheme, so we strip it; only "https://" implies TLS.
func parseEndpoint(endpoint string) (target string, insecure bool) {
	switch {
	case strings.HasPrefix(endpoint, "https://"):
		return strings.TrimPrefix(endpoint, "https://"), false
	case strings.HasPrefix(endpoint, "http://"):
		return strings.TrimPrefix(endpoint, "http://"), true
	default:
		return endpoint, true
	}
}
