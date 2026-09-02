package exporter_test

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/client-go/rest"

	"github.com/hostwithquantum/k8s-storage-exporter/internal/exporter"
)

func newCollector(t *testing.T, summaries map[string]string, node string) *exporter.Collector {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	c, err := exporter.New(apiServer(t, summaries), node, 5*time.Second, nil, "", log)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCollectSingleNode(t *testing.T) {
	c := newCollector(t, map[string]string{"node-a": nodeSummary}, "node-a")
	if err := testutil.CollectAndCompare(c, strings.NewReader(expectedMetrics)); err != nil {
		t.Error(err)
	}
	if problems, err := testutil.CollectAndLint(c); err != nil {
		t.Error(err)
	} else if len(problems) > 0 {
		t.Errorf("lint problems: %v", problems)
	}
}

func TestCollectAllNodes(t *testing.T) {
	// No node configured: the collector lists nodes and scrapes all of
	// them. Both report the same pod (rescheduling); it must be deduped.
	c := newCollector(t, map[string]string{
		"node-a": nodeSummary,
		"node-b": nodeSummary,
	}, "")

	if count := testutil.CollectAndCount(c, "storage_ephemeral_used_bytes"); count != 1 {
		t.Errorf("got %d storage_ephemeral_used_bytes metrics, want 1", count)
	}
}

func TestCollectCountsFailedNodes(t *testing.T) {
	c := newCollector(t, map[string]string{
		"node-a":      nodeSummary,
		"node-broken": "",
	}, "")

	expected := `
# HELP storage_scrape_errors Number of nodes that failed to be scraped during the last collection.
# TYPE storage_scrape_errors gauge
storage_scrape_errors 1
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), "storage_scrape_errors"); err != nil {
		t.Error(err)
	}
	if count := testutil.CollectAndCount(c, "storage_ephemeral_used_bytes"); count != 1 {
		t.Errorf("healthy node should still be scraped, got %d metrics, want 1", count)
	}
}

func TestCollectPodLabels(t *testing.T) {
	// web-0 is in the label cache; ghost is not (e.g. deleted between the
	// kubelet report and the watch) and must get empty label values.
	summary := `{"pods": [
		{"podRef": {"name": "web-0", "namespace": "demo"}, "ephemeral-storage": {"usedBytes": 1000}},
		{"podRef": {"name": "ghost", "namespace": "demo"}, "ephemeral-storage": {"usedBytes": 7}}
	]}`
	client := apiServer(t, map[string]string{"node-a": summary},
		`{"metadata": {"name": "web-0", "namespace": "demo",
		  "labels": {"app": "web", "app.kubernetes.io/name": "frontend", "ignored": "x"}}}`)

	c, err := exporter.New(client, "node-a", 5*time.Second,
		[]string{"app", "app.kubernetes.io/name"}, "", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	c.Run(t.Context())

	expected := `
# HELP storage_ephemeral_used_bytes Bytes used on the pod's ephemeral storage.
# TYPE storage_ephemeral_used_bytes gauge
storage_ephemeral_used_bytes{label_app="web",label_app_kubernetes_io_name="frontend",namespace="demo",pod="web-0"} 1000
storage_ephemeral_used_bytes{label_app="",label_app_kubernetes_io_name="",namespace="demo",pod="ghost"} 7
# HELP storage_pod_labels_cache_synced Whether the pod label cache behind --pod-labels is synced with the API server (1) or label values may be missing or stale (0).
# TYPE storage_pod_labels_cache_synced gauge
storage_pod_labels_cache_synced 1
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"storage_ephemeral_used_bytes", "storage_pod_labels_cache_synced"); err != nil {
		t.Error(err)
	}
}

func TestNewRejectsBadPodLabelKeys(t *testing.T) {
	client := apiServer(t, nil)
	for _, keys := range [][]string{
		{""},
		{"app", "app"},
		{"app.x", "app-x"}, // both sanitize to label_app_x
	} {
		if _, err := exporter.New(client, "", time.Second, keys, "", slog.New(slog.DiscardHandler)); err == nil {
			t.Errorf("keys %q: expected an error", keys)
		}
	}
}

func TestCollectPodSelector(t *testing.T) {
	// Both pods exist, but the selector keeps other-0 out of the label
	// cache, so its metrics get empty label values.
	summary := `{"pods": [
		{"podRef": {"name": "web-0", "namespace": "demo"}, "ephemeral-storage": {"usedBytes": 1000}},
		{"podRef": {"name": "other-0", "namespace": "demo"}, "ephemeral-storage": {"usedBytes": 7}}
	]}`
	client := apiServer(t, map[string]string{"node-a": summary},
		`{"metadata": {"name": "web-0", "namespace": "demo", "labels": {"team": "core", "app": "web"}}}`,
		`{"metadata": {"name": "other-0", "namespace": "demo", "labels": {"team": "other", "app": "other"}}}`)

	c, err := exporter.New(client, "node-a", 5*time.Second,
		[]string{"app"}, "team=core", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	c.Run(t.Context())

	expected := `
# HELP storage_ephemeral_used_bytes Bytes used on the pod's ephemeral storage.
# TYPE storage_ephemeral_used_bytes gauge
storage_ephemeral_used_bytes{label_app="web",namespace="demo",pod="web-0"} 1000
storage_ephemeral_used_bytes{label_app="",namespace="demo",pod="other-0"} 7
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), "storage_ephemeral_used_bytes"); err != nil {
		t.Error(err)
	}
}

func TestCollectListNodesError(t *testing.T) {
	// Empty node map: the fake returns an empty list, so break listing
	// instead with an unreachable server.
	c, err := exporter.New(&rest.Config{Host: "http://127.0.0.1:1"}, "", time.Second, nil, "", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}

	expected := `
# HELP storage_scrape_errors Number of nodes that failed to be scraped during the last collection.
# TYPE storage_scrape_errors gauge
storage_scrape_errors 1
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), "storage_scrape_errors"); err != nil {
		t.Error(err)
	}
}
