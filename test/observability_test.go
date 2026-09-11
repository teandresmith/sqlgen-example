package test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/teandresmith/sqlgen-example/internal/server"
)

// This file proves observability wiring (issue 15 / PRD story 56) through the
// same seam every other suite uses: the fully-wired HTTP handler the binary
// serves. It drives real traffic — a GraphQL query that exercises the client's
// cache, plus a health check — then snapshots the shared ManualReader the
// harness wired into both the HTTP request middleware and the cache's OTel
// recorder, and asserts the request and query metrics were recorded.
//
// The reader is a ManualReader rather than an OTLP exporter so the test needs
// no collector: it reads the exact instruments that, in production, the
// PeriodicReader ships to the docker-compose collector. Only the exporter
// differs between this test and the binary; the recording path is identical.

// TestRequestMetricsRecorded proves the HTTP request middleware records
// http.server.request.duration labeled by route and method — the "attached to
// the HTTP server" half of story 56.
func TestRequestMetricsRecorded(t *testing.T) {
	// A GraphQL POST (route /query) and a health GET (route /healthz) give two
	// distinct route labels to assert on.
	token, _ := register(t, "obs-req@example.com", "obs-pw-0000001", "Obs Req")
	wsID := bootstrapWorkspace(t, token, "ObsReq", "obs-req-ws")
	_ = createProject(t, token, wsID, "Observed work")

	resp, err := http.Get(testServer.URL + server.HealthPath)
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	_ = resp.Body.Close()

	m := findMetric(t, "http.server.request.duration")
	if m == nil {
		t.Fatal("http.server.request.duration not recorded; request middleware not wired")
	}
	hist, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("http.server.request.duration aggregation = %T, want Histogram[float64]", m.Data)
	}

	if !histogramHasAttrs(hist, map[string]string{"http.route": server.GraphQLPath, "http.request.method": http.MethodPost}) {
		t.Errorf("no POST %s request duration recorded", server.GraphQLPath)
	}
	if !histogramHasAttrs(hist, map[string]string{"http.route": server.HealthPath, "http.request.method": http.MethodGet}) {
		t.Errorf("no GET %s request duration recorded", server.HealthPath)
	}
}

// TestQueryMetricsRecorded proves the OTel cache recorder attached to the
// client records query-path metrics (sqlgen.cache.*) — the "attached to the
// client" half of story 56. Creating a Task warms its cache entry and a
// single-entity read exercises the cached Get, so at least one cache
// instrument must have a data point.
func TestQueryMetricsRecorded(t *testing.T) {
	token, _ := register(t, "obs-query@example.com", "obs-pw-0000002", "Obs Query")
	wsID := bootstrapWorkspace(t, token, "ObsQuery", "obs-query-ws")
	projID := createProject(t, token, wsID, "Cached observe")
	task := createTaskInProject(t, token, wsID, projID, "observe-me", "TODO", "MEDIUM", "")

	// Two reads of the same entity drive the cache Get path (warmed on create,
	// then a hit), so hit/set/miss counters are populated regardless of exact
	// ordering.
	_ = getTaskTitle(t, token, wsID, task.ID, nil)
	_ = getTaskTitle(t, token, wsID, task.ID, nil)

	// Any sqlgen.cache.* instrument having recorded proves the recorder is wired
	// into the client and exporting query-path metrics.
	rm := collectMetrics(t)
	var found string
	for i := range rm.ScopeMetrics {
		for _, mm := range rm.ScopeMetrics[i].Metrics {
			if strings.HasPrefix(mm.Name, "sqlgen.cache.") {
				found = mm.Name
				break
			}
		}
	}
	if found == "" {
		t.Fatal("no sqlgen.cache.* metric recorded; OTel cache recorder not attached to the client")
	}
}

// collectMetrics snapshots every instrument the harness MeterProvider has
// recorded against its ManualReader.
func collectMetrics(t *testing.T) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := testMetricReader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collecting metrics: %v", err)
	}
	return rm
}

// findMetric returns the recorded metric with the given name, or nil.
func findMetric(t *testing.T, name string) *metricdata.Metrics {
	t.Helper()
	rm := collectMetrics(t)
	for i := range rm.ScopeMetrics {
		for j := range rm.ScopeMetrics[i].Metrics {
			if rm.ScopeMetrics[i].Metrics[j].Name == name {
				return &rm.ScopeMetrics[i].Metrics[j]
			}
		}
	}
	return nil
}

// histogramHasAttrs reports whether any histogram data point carries every
// wanted attribute key/value pair.
func histogramHasAttrs(h metricdata.Histogram[float64], want map[string]string) bool {
	for _, dp := range h.DataPoints {
		match := true
		for k, v := range want {
			got, ok := dp.Attributes.Value(attribute.Key(k))
			if !ok || got.AsString() != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
