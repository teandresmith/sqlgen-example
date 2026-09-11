package observability

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/teandresmith/sqlgen/cache"
	sqlgenotel "github.com/teandresmith/sqlgen/metrics/otel"
)

// httpMeterName scopes the HTTP request instruments; it follows the
// OpenTelemetry convention of naming the meter after the emitting package.
const httpMeterName = "github.com/teandresmith/sqlgen-example/internal/observability"

// HTTP request metric attribute keys and instrument name. The route label is
// the matched mux pattern (e.g. "/query"), not the raw path, so cardinality
// stays bounded regardless of query strings or ids.
const (
	instrRequestDuration = "http.server.request.duration"

	attrKeyMethod = "http.request.method"
	attrKeyRoute  = "http.route"
	attrKeyStatus = "http.response.status_code"
)

// CacheRecorder returns the sqlgen OTel cache.MetricsRecorder wired to this
// Provider's MeterProvider — the "attached to the client" half of story 56.
// Attach the returned recorder to the generated cache with
// database.WithMetricsRecorder so cache hit/miss/latency (the query-path
// metrics) export to the collector.
func (p *Provider) CacheRecorder() cache.MetricsRecorder {
	return sqlgenotel.New(p.MeterProvider())
}

// Middleware wraps next with HTTP request instrumentation — the "attached to
// the HTTP server" half of story 56. It records the http.server.request.duration
// histogram (seconds) labeled by method, matched route, and response status,
// from which request rate and error rate are both derivable. Instruments are
// built once from mp and shared across requests.
//
// The route label is read from the request pattern the serving mux matched
// (Go 1.22+ ServeMux sets it), so it is the stable template ("/query") rather
// than the concrete path — wrap the mux, not each leaf handler, to get it.
func Middleware(mp metric.MeterProvider) func(http.Handler) http.Handler {
	meter := mp.Meter(httpMeterName)
	// OpenTelemetry guarantees a usable (no-op on error) instrument, so the
	// construction error is safe to ignore, matching the sqlgen OTel recorder.
	duration, _ := meter.Float64Histogram(
		instrRequestDuration,
		metric.WithUnit("s"),
		metric.WithDescription("Duration of inbound HTTP requests, by method, route, and status."),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sr, r)

			route := r.Pattern
			if route == "" {
				// No route matched (404) or served outside a pattern-aware mux;
				// bucket under a constant so an unrouted flood cannot explode
				// label cardinality with arbitrary paths.
				route = "unmatched"
			}
			// Record with the request context so OTel can correlate the reading
			// with any trace/baggage on it. The measurement is taken regardless
			// of whether the context is already cancelled by request completion.
			duration.Record(r.Context(), time.Since(start).Seconds(), metric.WithAttributes(
				attribute.String(attrKeyMethod, r.Method),
				attribute.String(attrKeyRoute, route),
				attribute.Int(attrKeyStatus, sr.status),
			))
		})
	}
}

// statusRecorder captures the response status code so it can be recorded as a
// metric label. It defaults to 200 because a handler that writes a body
// without an explicit WriteHeader implicitly sends 200.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	s.wroteHeader = true
	return s.ResponseWriter.Write(b)
}

// Flush forwards to the underlying ResponseWriter when it supports flushing, so
// wrapping the mux does not silently swallow streaming responses (e.g. gqlgen's
// deferred/multipart transport). It is a no-op when the underlying writer is
// not a Flusher. Hijack is deliberately not forwarded: GraphQL subscriptions /
// websocket upgrades are out of scope (PRD), so a wrapper that does not claim
// http.Hijacker lets such a request fail cleanly rather than pretend support.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
