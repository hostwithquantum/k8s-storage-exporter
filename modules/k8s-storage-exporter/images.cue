package main

// The container image published by the release workflow.
values: {
	image: {
		repository: *"ghcr.io/hostwithquantum/k8s-storage-exporter" | string
		tag:        *"latest" | string
		digest:     *"" | string
	}
}
