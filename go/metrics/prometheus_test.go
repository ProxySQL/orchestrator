package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestPrometheusMetricsRegisterWithoutPanic(t *testing.T) {
	// Create a new registry to avoid conflicts with default registry
	reg := prometheus.NewRegistry()

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_discoveries_total",
		Help: "Test counter.",
	})
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_instances_total",
		Help: "Test gauge.",
	})
	counterVec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_recoveries_total",
		Help: "Test counter vec.",
	}, []string{"type", "result"})
	histogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "test_recovery_duration_seconds",
		Help:    "Test histogram.",
		Buckets: prometheus.ExponentialBuckets(0.5, 2, 12),
	})

	reg.MustRegister(counter, gauge, counterVec, histogram)
}

func TestPrometheusCounterIncrement(t *testing.T) {
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_counter_inc",
		Help: "Test counter for increment.",
	})

	counter.Inc()
	counter.Inc()
	counter.Add(3)

	m := &dto.Metric{}
	if err := counter.Write(m); err != nil {
		t.Fatalf("unexpected error writing metric: %v", err)
	}
	if got := m.GetCounter().GetValue(); got != 5.0 {
		t.Errorf("expected counter value 5, got %f", got)
	}
}

func TestPrometheusGaugeSetAndGet(t *testing.T) {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_gauge_set",
		Help: "Test gauge for set.",
	})

	gauge.Set(42)

	m := &dto.Metric{}
	if err := gauge.Write(m); err != nil {
		t.Fatalf("unexpected error writing metric: %v", err)
	}
	if got := m.GetGauge().GetValue(); got != 42.0 {
		t.Errorf("expected gauge value 42, got %f", got)
	}
}

func TestPrometheusHistogramObserve(t *testing.T) {
	hist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "test_histogram_observe",
		Help:    "Test histogram for observe.",
		Buckets: prometheus.DefBuckets,
	})

	hist.Observe(0.5)
	hist.Observe(1.5)
	hist.Observe(3.0)

	m := &dto.Metric{}
	if err := hist.Write(m); err != nil {
		t.Fatalf("unexpected error writing metric: %v", err)
	}
	if got := m.GetHistogram().GetSampleCount(); got != 3 {
		t.Errorf("expected 3 samples, got %d", got)
	}
}

func TestPrometheusCounterVecLabels(t *testing.T) {
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_counter_vec_labels",
		Help: "Test counter vec with labels.",
	}, []string{"type", "result"})

	cv.WithLabelValues("DeadMaster", "success").Inc()
	cv.WithLabelValues("DeadMaster", "failure").Add(2)

	m := &dto.Metric{}
	c, err := cv.GetMetricWithLabelValues("DeadMaster", "success")
	if err != nil {
		t.Fatalf("unexpected error getting metric: %v", err)
	}
	if err := c.Write(m); err != nil {
		t.Fatalf("unexpected error writing metric: %v", err)
	}
	if got := m.GetCounter().GetValue(); got != 1.0 {
		t.Errorf("expected counter value 1, got %f", got)
	}
}
