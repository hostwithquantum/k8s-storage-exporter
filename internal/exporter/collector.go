// Package exporter exposes pod storage stats from the kubelet
// stats/summary API as Prometheus metrics.
package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Collector reads the kubelet stats/summary of one node (or all nodes, if
// node is empty) through the API server proxy on each Prometheus scrape and
// turns pod storage stats into metrics.
type Collector struct {
	client  *kubernetes.Clientset
	node    string
	timeout time.Duration
	log     *slog.Logger
}

func New(client *kubernetes.Clientset, node string, timeout time.Duration, log *slog.Logger) *Collector {
	return &Collector{client: client, node: node, timeout: timeout, log: log}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, descs := range []fsDescs{ephemeralDescs, volumeDescs} {
		ch <- descs.used
		ch <- descs.available
		ch <- descs.capacity
		ch <- descs.inodes
		ch <- descs.inodesFree
		ch <- descs.inodesUsed
	}
	ch <- scrapeErrors
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	nodes, err := c.nodes(ctx)
	if err != nil {
		c.log.Error("listing nodes failed", "error", err)
		ch <- prometheus.MustNewConstMetric(scrapeErrors, prometheus.GaugeValue, 1)
		return
	}

	var (
		mu     sync.Mutex
		seen   = make(map[string]bool)
		failed int
		wg     sync.WaitGroup
	)
	for _, node := range nodes {
		wg.Go(func() {
			pods, err := c.scrapeNode(ctx, node)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				c.log.Error("scraping node failed", "node", node, "error", err)
				failed++
				return
			}
			c.emit(ch, pods, seen)
		})
	}
	wg.Wait()

	ch <- prometheus.MustNewConstMetric(scrapeErrors, prometheus.GaugeValue, float64(failed))
}

func (c *Collector) nodes(ctx context.Context) ([]string, error) {
	if c.node != "" {
		return []string{c.node}, nil
	}
	list, err := c.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, node := range list.Items {
		names = append(names, node.Name)
	}
	return names, nil
}

func (c *Collector) scrapeNode(ctx context.Context, node string) ([]podStats, error) {
	raw, err := c.client.CoreV1().RESTClient().Get().
		Resource("nodes").Name(node).
		SubResource("proxy").Suffix("stats/summary").
		Do(ctx).Raw()
	if err != nil {
		return nil, err
	}
	var s summary
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("decoding stats: %w", err)
	}
	return s.Pods, nil
}

// emit sends metrics for each pod. A pod is only reported once, even if two
// nodes briefly report it during rescheduling.
func (c *Collector) emit(ch chan<- prometheus.Metric, pods []podStats, seen map[string]bool) {
	for _, pod := range pods {
		key := pod.PodRef.Namespace + "/" + pod.PodRef.Name
		if seen[key] {
			continue
		}
		seen[key] = true

		if fs := pod.EphemeralStorage; fs != nil {
			sendFS(ch, fs, ephemeralDescs, pod.PodRef.Name, pod.PodRef.Namespace)
		}
		for _, vol := range pod.Volumes {
			sendFS(ch, &vol.fsStats, volumeDescs, pod.PodRef.Name, pod.PodRef.Namespace, vol.Name)
		}
	}
}
