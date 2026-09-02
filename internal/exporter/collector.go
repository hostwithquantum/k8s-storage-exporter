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
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// Collector reads the kubelet stats/summary of one node (or all nodes, if
// node is empty) through the API server proxy on each Prometheus scrape and
// turns pod storage stats into metrics.
type Collector struct {
	client  *kubernetes.Clientset
	node    string
	timeout time.Duration
	log     *slog.Logger

	labels    *podLabels // nil unless pod label keys are configured
	ephemeral fsDescs
	volumes   fsDescs
}

// New builds a Collector on the given API server config. podLabelKeys
// names the pod labels to attach to every metric (as label_<key>); when
// non-empty, Run must be called to keep the label cache warm. podSelector
// is a label selector limiting which pods are watched for that cache; pods
// it filters out still get metrics, just with empty label values.
func New(cfg *rest.Config, node string, timeout time.Duration, podLabelKeys []string, podSelector string, log *slog.Logger) (*Collector, error) {
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	c := &Collector{client: client, node: node, timeout: timeout, log: log}
	extra, err := metricLabelNames(podLabelKeys)
	if err != nil {
		return nil, fmt.Errorf("invalid pod labels: %w", err)
	}
	c.ephemeral = newFSDescs("storage_ephemeral_",
		"the pod's ephemeral storage", append([]string{"pod", "namespace"}, extra...))
	c.volumes = newFSDescs("storage_volumes_",
		"the volume", append([]string{"pod", "namespace", "volume"}, extra...))
	if len(podLabelKeys) > 0 {
		metaClient, err := metadata.NewForConfig(cfg)
		if err != nil {
			return nil, err
		}
		c.labels = newPodLabels(metaClient, node, podSelector, podLabelKeys)
	}
	return c, nil
}

// labelSyncTimeout bounds how long Run waits for the initial label cache
// sync. Serving must not be blocked forever by a broken watch (e.g.
// missing RBAC); after the timeout the exporter serves metrics with empty
// label values and the watch keeps retrying in the background, visible as
// storage_pod_labels_cache_synced 0.
const labelSyncTimeout = 30 * time.Second

// Run starts the pod watch feeding the label cache and waits for its
// initial sync, so the first scrape already has labels. The watch stops
// when ctx is canceled. Without pod label keys this is a no-op.
func (c *Collector) Run(ctx context.Context) {
	if c.labels == nil {
		return
	}
	c.log.Info("watching pods for labels", "labels", c.labels.keys)
	go c.labels.informer.Run(ctx.Done())

	syncCtx, cancel := context.WithTimeout(ctx, labelSyncTimeout)
	defer cancel()
	if !cache.WaitForCacheSync(syncCtx.Done(), c.labels.informer.HasSynced) {
		c.log.Error("pod label cache not synced, serving metrics with empty pod labels until it catches up")
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, descs := range []fsDescs{c.ephemeral, c.volumes} {
		ch <- descs.used
		ch <- descs.available
		ch <- descs.capacity
		ch <- descs.inodes
		ch <- descs.inodesFree
		ch <- descs.inodesUsed
	}
	ch <- scrapeErrors
	if c.labels != nil {
		ch <- labelsSynced
	}
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
	if c.labels != nil {
		synced := 0.0
		if c.labels.informer.HasSynced() {
			synced = 1
		}
		ch <- prometheus.MustNewConstMetric(labelsSynced, prometheus.GaugeValue, synced)
	}
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

		var extra []string
		if c.labels != nil {
			extra = c.labels.values(pod.PodRef.Namespace, pod.PodRef.Name)
		}
		if fs := pod.EphemeralStorage; fs != nil {
			sendFS(ch, fs, c.ephemeral, append([]string{pod.PodRef.Name, pod.PodRef.Namespace}, extra...)...)
		}
		if len(pod.Volumes) == 0 {
			continue
		}
		// One slice per pod, the volume slot is overwritten per volume;
		// sendFS copies the values into the metric.
		values := append([]string{pod.PodRef.Name, pod.PodRef.Namespace, ""}, extra...)
		for _, vol := range pod.Volumes {
			values[2] = vol.Name
			sendFS(ch, &vol.fsStats, c.volumes, values...)
		}
	}
}
