package templates

import (
	rbacv1 "k8s.io/api/rbac/v1"
)

// The ClusterRole and ClusterRoleBinding are cluster-scoped; their names
// include the namespace so instances in different namespaces don't clash.

#ClusterRole: rbacv1.#ClusterRole & {
	#config:    #Config
	apiVersion: "rbac.authorization.k8s.io/v1"
	kind:       "ClusterRole"
	metadata: {
		name:   "\(#config.metadata.name)-\(#config.metadata.namespace)"
		labels: #config.metadata.labels
		if #config.metadata.annotations != _|_ {
			annotations: #config.metadata.annotations
		}
	}
	rules: [
		{
			apiGroups: [""]
			resources: ["nodes"]
			verbs: ["get", "list"]
		},
		{
			apiGroups: [""]
			resources: ["nodes/proxy"]
			verbs: ["get"]
		},
	]
}

#ClusterRoleBinding: rbacv1.#ClusterRoleBinding & {
	#config:    #Config
	apiVersion: "rbac.authorization.k8s.io/v1"
	kind:       "ClusterRoleBinding"
	metadata: {
		name:   "\(#config.metadata.name)-\(#config.metadata.namespace)"
		labels: #config.metadata.labels
		if #config.metadata.annotations != _|_ {
			annotations: #config.metadata.annotations
		}
	}
	roleRef: {
		apiGroup: "rbac.authorization.k8s.io"
		kind:     "ClusterRole"
		name:     "\(#config.metadata.name)-\(#config.metadata.namespace)"
	}
	subjects: [{
		kind:      "ServiceAccount"
		name:      #config.metadata.name
		namespace: #config.metadata.namespace
	}]
}
