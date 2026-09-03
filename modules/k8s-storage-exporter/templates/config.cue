package templates

import (
	corev1 "k8s.io/api/core/v1"
	timoniv1 "timoni.sh/core/v1alpha1"
)

// Values defines the user-supplied values schema: only the knobs a user
// should care about. Everything else lives in #Config.
#Values: {
	// The image allows setting the container image repository,
	// tag, digest and pull policy.
	// The default image repository and tag are set in `images.cue`.
	image!: timoniv1.#Image

	// The resources allows setting the container resource requirements.
	resources: timoniv1.#ResourceRequirements & {
		requests: {
			cpu:    *"10m" | timoniv1.#CPUQuantity
			memory: *"32Mi" | timoniv1.#MemoryQuantity
		}
		limits: {
			memory: *"64Mi" | timoniv1.#MemoryQuantity
		}
	}

	// Pod optional settings.
	imagePullSecrets?: [...timoniv1.#ObjectReference]

	// The exporter runs on every node by default, including control plane.
	tolerations: *[{operator: "Exists"}] | [...corev1.#Toleration]

	// Pods are scheduled on Linux nodes by default.
	nodeSelector: *{"kubernetes.io/os": "linux"} | {[string]: string}

	// App settings.

	// The port the metrics endpoint listens on.
	listenPort: *9110 | int & >0 & <=65535

	// Timeout for collecting stats from the kubelet.
	scrapeTimeout: *"30s" | string

	// Exclude metrics about the exporter itself (go_*, process_*).
	disableExporterMetrics: *false | bool

	// Pod label keys attached to metrics as label_<key>. When set, the
	// exporter watches pods and the ClusterRole gains pods list/watch.
	podLabels: *[] | [...string]

	// Label selector (e.g. "team=core") limiting which pods are watched
	// for podLabels; other pods get empty label values.
	podSelector: *"" | string

	// The binary rejects --pod-selector without --pod-labels; require
	// podLabels here so timoni fails before deploying.
	if podSelector != "" {
		podLabels: [string, ...string]
	}

	// The pod annotations; by default the Prometheus scrape annotations.
	podAnnotations: *{
		"prometheus.io/scrape": "true"
		"prometheus.io/port":   "\(listenPort)"
		"prometheus.io/path":   "/metrics"
	} | {[string]: string}
}

// Config is the values plus the runtime fields injected by Timoni.
#Config: {
	#Values

	// The moduleVersion is set from the user-supplied module version.
	// This field is used for the `app.kubernetes.io/version` label.
	moduleVersion!: string

	// The Kubernetes metadata common to all resources.
	// The `metadata.name` and `metadata.namespace` fields are
	// set from the user-supplied instance name and namespace.
	metadata: timoniv1.#Metadata & {#Version: moduleVersion}
	metadata: labels: timoniv1.#Labels
	metadata: annotations?: timoniv1.#Annotations

	// The label selector, generated from the instance name.
	selector: timoniv1.#Selector & {#Name: metadata.name}
}

// Instance takes the config values and outputs the Kubernetes objects.
#Instance: {
	config: #Config

	objects: {
		sa: #ServiceAccount & {#config: config}
		clusterrole: #ClusterRole & {#config: config}
		clusterrolebinding: #ClusterRoleBinding & {#config: config}
		daemonset: #DaemonSet & {#config: config}
	}
}
