package exporter_test

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/hostwithquantum/k8s-storage-exporter/internal/exporter"
)

func newCollector(t *testing.T, summaries map[string]string, node string) *exporter.Collector {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	return exporter.New(apiServer(t, summaries), node, 5*time.Second, log)
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

func TestCollectListNodesError(t *testing.T) {
	// Empty node map: the fake returns an empty list, so break listing
	// instead with an unreachable server.
	client, err := kubernetes.NewForConfig(&rest.Config{Host: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	c := exporter.New(client, "", time.Second, slog.New(slog.DiscardHandler))

	expected := `
# HELP storage_scrape_errors Number of nodes that failed to be scraped during the last collection.
# TYPE storage_scrape_errors gauge
storage_scrape_errors 1
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), "storage_scrape_errors"); err != nil {
		t.Error(err)
	}
}
