package templates

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

#DaemonSet: appsv1.#DaemonSet & {
	#config: #Config

	apiVersion: "apps/v1"
	kind:       "DaemonSet"
	metadata:   #config.metadata
	spec: appsv1.#DaemonSetSpec & {
		selector: matchLabels: #config.selector.labels
		template: {
			metadata: {
				labels:      #config.selector.labels
				annotations: #config.podAnnotations
			}
			spec: corev1.#PodSpec & {
				serviceAccountName: #config.metadata.name
				containers: [
					{
						name:            #config.metadata.name
						image:           #config.image.reference
						imagePullPolicy: #config.image.pullPolicy
						args: [
							"--node=$(NODE_NAME)",
							"--listen-address=:\(#config.listenPort)",
							"--scrape-timeout=\(#config.scrapeTimeout)",
							if #config.disableExporterMetrics {
								"--disable-exporter-metrics"
							},
							if len(#config.podLabels) > 0 {
								"--pod-labels=" + strings.Join(#config.podLabels, ",")
							},
							if #config.podSelector != "" {
								"--pod-selector=" + #config.podSelector
							},
						]
						env: [{
							name: "NODE_NAME"
							valueFrom: fieldRef: fieldPath: "spec.nodeName"
						}]
						ports: [{
							name:          "metrics"
							containerPort: #config.listenPort
							protocol:      "TCP"
						}]
						readinessProbe: httpGet: {
							path: "/healthz"
							port: "metrics"
						}
						livenessProbe: httpGet: {
							path: "/healthz"
							port: "metrics"
						}
						resources:       #config.resources
						securityContext: #config.securityContext
					},
				]
				if #config.podSecurityContext != _|_ {
					securityContext: #config.podSecurityContext
				}
				nodeSelector: #config.nodeSelector
				tolerations:  #config.tolerations
				if #config.imagePullSecrets != _|_ {
					imagePullSecrets: #config.imagePullSecrets
				}
			}
		}
	}
}
