package exporter_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const nodeSummary = `{
  "pods": [
    {
      "podRef": {"name": "web-0", "namespace": "demo"},
      "ephemeral-storage": {
        "availableBytes": 2000,
        "capacityBytes": 3000,
        "usedBytes": 1000,
        "inodesFree": 280,
        "inodes": 300,
        "inodesUsed": 20
      },
      "volume": [
        {"name": "data", "usedBytes": 500, "availableBytes": 1500, "capacityBytes": 2000,
         "inodes": 100, "inodesFree": 90, "inodesUsed": 10},
        {"name": "logs", "usedBytes": 700}
      ]
    },
    {
      "podRef": {"name": "no-stats", "namespace": "empty"}
    }
  ]
}`

const expectedMetrics = `
# HELP storage_ephemeral_available_bytes Bytes available on the pod's ephemeral storage.
# TYPE storage_ephemeral_available_bytes gauge
storage_ephemeral_available_bytes{namespace="demo",pod="web-0"} 2000
# HELP storage_ephemeral_capacity_bytes Capacity of the pod's ephemeral storage, in bytes.
# TYPE storage_ephemeral_capacity_bytes gauge
storage_ephemeral_capacity_bytes{namespace="demo",pod="web-0"} 3000
# HELP storage_ephemeral_inodes Total inodes on the pod's ephemeral storage.
# TYPE storage_ephemeral_inodes gauge
storage_ephemeral_inodes{namespace="demo",pod="web-0"} 300
# HELP storage_ephemeral_inodes_free Free inodes on the pod's ephemeral storage.
# TYPE storage_ephemeral_inodes_free gauge
storage_ephemeral_inodes_free{namespace="demo",pod="web-0"} 280
# HELP storage_ephemeral_inodes_used Used inodes on the pod's ephemeral storage.
# TYPE storage_ephemeral_inodes_used gauge
storage_ephemeral_inodes_used{namespace="demo",pod="web-0"} 20
# HELP storage_ephemeral_used_bytes Bytes used on the pod's ephemeral storage.
# TYPE storage_ephemeral_used_bytes gauge
storage_ephemeral_used_bytes{namespace="demo",pod="web-0"} 1000
# HELP storage_scrape_errors Number of nodes that failed to be scraped during the last collection.
# TYPE storage_scrape_errors gauge
storage_scrape_errors 0
# HELP storage_volumes_available_bytes Bytes available on the volume.
# TYPE storage_volumes_available_bytes gauge
storage_volumes_available_bytes{namespace="demo",pod="web-0",volume="data"} 1500
# HELP storage_volumes_capacity_bytes Capacity of the volume, in bytes.
# TYPE storage_volumes_capacity_bytes gauge
storage_volumes_capacity_bytes{namespace="demo",pod="web-0",volume="data"} 2000
# HELP storage_volumes_inodes Total inodes on the volume.
# TYPE storage_volumes_inodes gauge
storage_volumes_inodes{namespace="demo",pod="web-0",volume="data"} 100
# HELP storage_volumes_inodes_free Free inodes on the volume.
# TYPE storage_volumes_inodes_free gauge
storage_volumes_inodes_free{namespace="demo",pod="web-0",volume="data"} 90
# HELP storage_volumes_inodes_used Used inodes on the volume.
# TYPE storage_volumes_inodes_used gauge
storage_volumes_inodes_used{namespace="demo",pod="web-0",volume="data"} 10
# HELP storage_volumes_used_bytes Bytes used on the volume.
# TYPE storage_volumes_used_bytes gauge
storage_volumes_used_bytes{namespace="demo",pod="web-0",volume="data"} 500
storage_volumes_used_bytes{namespace="demo",pod="web-0",volume="logs"} 700
`

// apiServer fakes the two API server endpoints the exporter talks to.
// summaries maps node name -> stats/summary JSON; a missing node returns 500.
func apiServer(t *testing.T, summaries map[string]string) *kubernetes.Clientset {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/nodes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		items := make([]string, 0, len(summaries))
		for name := range summaries {
			items = append(items, `{"metadata": {"name": "`+name+`"}}`)
		}
		_, _ = w.Write([]byte(`{"apiVersion": "v1", "kind": "NodeList", "items": [` + strings.Join(items, ",") + `]}`))
	})
	mux.HandleFunc("GET /api/v1/nodes/{node}/proxy/stats/summary", func(w http.ResponseWriter, r *http.Request) {
		s := summaries[r.PathValue("node")]
		if s == "" {
			http.Error(w, "kubelet unreachable", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(s))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
