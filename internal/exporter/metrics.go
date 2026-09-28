package exporter

import "github.com/prometheus/client_golang/prometheus"

// fsDescs holds one metric descriptor per fsStats field.
type fsDescs struct {
	used, available, capacity      *prometheus.Desc
	inodes, inodesFree, inodesUsed *prometheus.Desc
}

func newFSDescs(prefix, subject string, labels []string) fsDescs {
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prefix+name, help, labels, nil)
	}
	return fsDescs{
		used:       desc("used_bytes", "Bytes used on "+subject+"."),
		available:  desc("available_bytes", "Bytes available on "+subject+"."),
		capacity:   desc("capacity_bytes", "Capacity of "+subject+", in bytes."),
		inodes:     desc("inodes", "Total inodes on "+subject+"."),
		inodesFree: desc("inodes_free", "Free inodes on "+subject+"."),
		inodesUsed: desc("inodes_used", "Used inodes on "+subject+"."),
	}
}

var (
	scrapeErrors = prometheus.NewDesc("storage_scrape_errors",
		"Whether scraping the node's kubelet stats failed during the last "+
			"collection (1) or not (0); reported without a node when listing "+
			"the nodes failed.", []string{"node"}, nil)

	labelsSynced = prometheus.NewDesc("storage_pod_labels_cache_synced",
		"Whether the pod label cache behind --pod-labels is synced with the "+
			"API server (1) or label values may be missing or stale (0).", nil, nil)
)

// containerFS combines a container's rootfs and logs usage into one
// fsStats: together they count toward the container's ephemeral-storage
// limit (https://kubernetes.io/docs/concepts/storage/ephemeral-storage/).
// available/capacity/inodes come from the same underlying filesystem stat
// on both, so rootfs's copy is kept as-is; only the used figures add up.
func containerFS(rootfs, logs *fsStats) *fsStats {
	if rootfs == nil {
		return logs
	}
	if logs == nil {
		return rootfs
	}
	out := *rootfs
	out.UsedBytes = addUint64(rootfs.UsedBytes, logs.UsedBytes)
	out.InodesUsed = addUint64(rootfs.InodesUsed, logs.InodesUsed)
	return &out
}

func addUint64(a, b *uint64) *uint64 {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	sum := *a + *b
	return &sum
}

func sendFS(ch chan<- prometheus.Metric, fs *fsStats, descs fsDescs, labels ...string) {
	send := func(desc *prometheus.Desc, value *uint64) {
		if value != nil {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(*value), labels...)
		}
	}
	send(descs.used, fs.UsedBytes)
	send(descs.available, fs.AvailableBytes)
	send(descs.capacity, fs.CapacityBytes)
	send(descs.inodes, fs.Inodes)
	send(descs.inodesFree, fs.InodesFree)
	send(descs.inodesUsed, fs.InodesUsed)
}
