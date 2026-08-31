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
	ephemeralDescs = newFSDescs("storage_ephemeral_",
		"the pod's ephemeral storage", []string{"pod", "namespace"})
	volumeDescs = newFSDescs("storage_volumes_",
		"the volume", []string{"pod", "namespace", "volume"})

	scrapeErrors = prometheus.NewDesc("storage_scrape_errors",
		"Number of nodes that failed to be scraped during the last collection.", nil, nil)
)

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
