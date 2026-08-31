# k8s-storage-exporter

Prometheus exporter for pod storage usage, based on the kubelet
`stats/summary` API. It exposes ephemeral storage and volume usage per pod.

## Metrics

Labels are the pod's `podRef.name` (as `pod`, matching kube-state-metrics
and cAdvisor for easy joins) and `podRef.namespace`; volume metrics also
carry the `volume` name.

| Metric                                              | Labels                                    |
| --------------------------------------------------- | ----------------------------------------- |
| `storage_ephemeral_{used,available,capacity}_bytes` | `pod`, `namespace`                        |
| `storage_ephemeral_inodes{,_free,_used}`            | `pod`, `namespace`                        |
| `storage_volumes_{used,available,capacity}_bytes`   | `pod`, `namespace`, `volume`              |
| `storage_volumes_inodes{,_free,_used}`              | `pod`, `namespace`, `volume`              |
| `storage_scrape_errors`                             | number of nodes that failed during scrape |

Statistics are fetched through the API server proxy (`/api/v1/nodes/<node>/proxy/stats/summary`). Auth is handled by client-go: in-cluster config in the cluster, kubeconfig (`--kubeconfig` or `$KUBECONFIG`) locally.

The exporter runs as a DaemonSet where each pod scrapes only its own node (`--node`, set from the downward API); without `--node` it scrapes all nodes. Deployment is done with the [Timoni](https://timoni.sh) module in `modules/k8s-storage-exporter` (DaemonSet plus the required RBAC):

```shell
timoni -n monitoring apply k8s-storage-exporter ./modules/k8s-storage-exporter
```

## Run

```shell
# local: needs $KUBECONFIG, scrapes all nodes
make run-dev

# flags
k8s-storage-exporter \
  --listen-address :9110 \
  --kubeconfig ./my.kubeconfig \  # $KUBECONFIG
  --node node-a \                 # $NODE_NAME, empty scrapes all nodes
  --scrape-timeout 30s \
  --disable-exporter-metrics      # drop go_* and process_* metrics
```

## Develop

```shell
make test
make lint
make build
```

## Release

Push a `v*` tag. GitHub Actions runs goreleaser, which builds linux
amd64/arm64 binaries and pushes a multi-arch image to
`ghcr.io/hostwithquantum/k8s-storage-exporter` via ko.
