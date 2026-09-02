package exporter

import (
	"fmt"
	"regexp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
)

// podLabels caches pod labels through a watch on the API server, so the
// collector can attach them to metrics without listing pods on every
// scrape. It watches pod metadata only (PartialObjectMetadata), so the API
// server never sends pod specs or statuses. The watch is limited to one
// node's pods when node is set, and to pods matching selector when
// selector is set; both filters are applied by the API server,
// filtered-out pods never reach the exporter.
type podLabels struct {
	keys     []string
	informer cache.SharedIndexInformer
}

var podsResource = schema.GroupVersionResource{Version: "v1", Resource: "pods"}

func newPodLabels(client metadata.Interface, node, selector string, keys []string) *podLabels {
	informer := metadatainformer.NewFilteredMetadataInformer(client, podsResource,
		metav1.NamespaceAll, 0, cache.Indexers{},
		func(o *metav1.ListOptions) {
			if node != "" {
				o.FieldSelector = "spec.nodeName=" + node
			}
			o.LabelSelector = selector
		}).Informer()
	p := &podLabels{keys: keys, informer: informer}
	// Keep only name, namespace and the wanted labels of each pod in the
	// cache; the rest of the metadata (managed fields, annotations, ...)
	// is dead weight.
	_ = p.informer.SetTransform(p.strip) // only errors after start
	return p
}

func (p *podLabels) strip(obj any) (any, error) {
	pod, ok := obj.(*metav1.PartialObjectMetadata)
	if !ok {
		return obj, nil
	}
	kept := make(map[string]string, len(p.keys))
	for _, key := range p.keys {
		if value, ok := pod.Labels[key]; ok {
			kept[key] = value
		}
	}
	return &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Labels:    kept,
	}}, nil
}

// values returns the pod's label values, aligned with keys. Pods missing
// from the cache (e.g. just deleted) get empty values.
func (p *podLabels) values(namespace, name string) []string {
	values := make([]string, len(p.keys))
	obj, exists, err := p.informer.GetIndexer().GetByKey(namespace + "/" + name)
	if err != nil || !exists {
		return values
	}
	pod, ok := obj.(*metav1.PartialObjectMetadata)
	if !ok {
		return values
	}
	for i, key := range p.keys {
		values[i] = pod.Labels[key]
	}
	return values
}

var invalidLabelChars = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// metricLabelNames turns pod label keys into Prometheus label names, e.g.
// "app.kubernetes.io/name" -> "label_app_kubernetes_io_name". Empty keys
// and keys that collide after sanitizing are rejected; they would build an
// invalid metric descriptor.
func metricLabelNames(keys []string) ([]string, error) {
	names := make([]string, len(keys))
	seen := make(map[string]string, len(keys))
	for i, key := range keys {
		if key == "" {
			return nil, fmt.Errorf("empty pod label key")
		}
		name := "label_" + invalidLabelChars.ReplaceAllString(key, "_")
		if prev, ok := seen[name]; ok {
			return nil, fmt.Errorf("pod label keys %q and %q both map to metric label %q", prev, key, name)
		}
		seen[name] = key
		names[i] = name
	}
	return names, nil
}
