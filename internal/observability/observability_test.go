package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestSetupDisabledWhenEndpointEmpty proves metric export is optional: an empty
// endpoint yields a usable no-op Provider whose MeterProvider is non-nil and
// whose Shutdown is a safe no-op, so the server runs without a collector.
func TestSetupDisabledWhenEndpointEmpty(t *testing.T) {
	p, err := Setup(context.Background(), "")
	if err != nil {
		t.Fatalf("Setup(\"\") error: %v", err)
	}
	if p.MeterProvider() == nil {
		t.Fatal("MeterProvider() is nil; instruments could not be built")
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown on no-op provider: %v", err)
	}
}

func TestParseEndpoint(t *testing.T) {
	cases := []struct {
		in           string
		wantTarget   string
		wantInsecure bool
	}{
		{"localhost:4317", "localhost:4317", true},
		{"http://otel-collector:4317", "otel-collector:4317", true},
		{"https://collector.example.com:4317", "collector.example.com:4317", false},
	}
	for _, c := range cases {
		gotTarget, gotInsecure := parseEndpoint(c.in)
		if gotTarget != c.wantTarget || gotInsecure != c.wantInsecure {
			t.Errorf("parseEndpoint(%q) = (%q, %v), want (%q, %v)",
				c.in, gotTarget, gotInsecure, c.wantTarget, c.wantInsecure)
		}
	}
}

// TestMiddlewareRecordsRequestDuration proves the middleware records the request
// duration histogram labeled with the matched route, method, and status code.
func TestMiddlewareRecordsRequestDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	mux := http.NewServeMux()
	mux.HandleFunc("/thing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := Middleware(mp)(mux)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/thing", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status passed through = %d, want %d", rec.Code, http.StatusTeapot)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	hist := findHistogram(t, rm, instrRequestDuration)
	if !pointHasAttrs(hist, map[string]any{
		attrKeyMethod: http.MethodGet,
		attrKeyRoute:  "/thing",
		attrKeyStatus: int64(http.StatusTeapot),
	}) {
		t.Errorf("no data point for GET /thing with status 418: %+v", hist.DataPoints)
	}
}

func findHistogram(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Histogram[float64] {
	t.Helper()
	for i := range rm.ScopeMetrics {
		for _, m := range rm.ScopeMetrics[i].Metrics {
			if m.Name == name {
				h, ok := m.Data.(metricdata.Histogram[float64])
				if !ok {
					t.Fatalf("metric %q = %T, want Histogram[float64]", name, m.Data)
				}
				return h
			}
		}
	}
	t.Fatalf("metric %q not recorded", name)
	return metricdata.Histogram[float64]{}
}

func pointHasAttrs(h metricdata.Histogram[float64], want map[string]any) bool {
	for _, dp := range h.DataPoints {
		match := true
		for k, v := range want {
			got, ok := dp.Attributes.Value(attribute.Key(k))
			if !ok {
				match = false
				break
			}
			switch want := v.(type) {
			case string:
				if got.AsString() != want {
					match = false
				}
			case int64:
				if got.AsInt64() != want {
					match = false
				}
			}
			if !match {
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
