# k8s-storage-exporter

A [Timoni](https://timoni.sh) module for deploying k8s-storage-exporter as
a DaemonSet: ServiceAccount, ClusterRole (+Binding) and the DaemonSet
itself. Each pod scrapes its own node's kubelet stats through the API
server proxy.

## Install

```shell
timoni -n monitoring apply k8s-storage-exporter ./modules/k8s-storage-exporter
```

To see the generated manifests without applying:

```shell
timoni -n monitoring build k8s-storage-exporter ./modules/k8s-storage-exporter
```

## Configuration

Custom values are passed with `--values ./my-values.cue`:

```cue
values: {
	disableExporterMetrics: true
	scrapeTimeout:          "10s"
}
```

| Key                       | Type                | Default                                       | Description                                            |
| ------------------------- | ------------------- | --------------------------------------------- | ------------------------------------------------------ |
| `image.repository`        | `string`            | `ghcr.io/hostwithquantum/k8s-storage-exporter`| Container image repository                             |
| `image.tag`               | `string`            | `latest`                                      | Container image tag                                    |
| `image.digest`            | `string`            | `""`                                          | Container image digest, takes precedence over tag      |
| `listenPort`              | `int`               | `9110`                                        | Port the metrics endpoint listens on                   |
| `scrapeTimeout`           | `string`            | `30s`                                         | Timeout for collecting stats from the kubelet          |
| `disableExporterMetrics`  | `bool`              | `false`                                       | Exclude `go_*` and `process_*` metrics                 |
| `podLabels`               | `[...string]`       | `[]`                                          | Pod label keys attached to metrics as `label_<key>`; adds pods list/watch RBAC |
| `podSelector`             | `string`            | `""`                                          | Label selector limiting which pods are watched for `podLabels` |
| `podAnnotations`          | `{[string]: string}`| Prometheus scrape annotations                 | Pod annotations                                        |
| `resources`               | object              | requests 10m/32Mi, limit 64Mi                 | Container resources                                    |
| `tolerations`             | list                | `[{operator: "Exists"}]`                      | Run on all nodes, including control plane              |
| `nodeSelector`            | `{[string]: string}`| `{"kubernetes.io/os": "linux"}`               | Node selector                                          |

Plus the standard Timoni fields: `metadata.labels`, `metadata.annotations`,
`securityContext`, `podSecurityContext`, `imagePullSecrets`.
