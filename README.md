# Kubernetes resource exporter

[![license](https://img.shields.io/github/license/webdevops/kube-resource-exporter.svg)](https://github.com/webdevops/kube-resource-exporter/blob/main/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fkube--resource--exporter-blue)](https://hub.docker.com/r/webdevops/kube-resource-exporter/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fkube--resource--exporter-blue)](https://quay.io/repository/webdevops/kube-resource-exporter)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kube-resource-exporter)](https://artifacthub.io/packages/search?repo=kube-resource-exporter)

Prometheus exporter for Kubernetes resources

Why not using [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics)?
Because they refused to generate custom metrics for group="" resources (Pods, Secrets, ...).

If you don't need custom metrics for group="" resources, try to use kube-state-metrics for custom resources.

## Usage

```
Usage:
  kube-resource-exporter [OPTIONS]

Application Options:
      --log.debug               debug mode [$LOG_DEBUG]
      --log.devel               development mode [$LOG_DEVEL]
      --log.json                Switch log output to json format [$LOG_JSON]
      --kubeconfig=             Kuberentes config path (should be empty if in-cluster) [$KUBECONFIG]
      --metric.label.name=      Label for resource name (default: name) [$METRIC_LABEL_NAME]
      --metric.label.namespace= Label for resource namespace (default: namespace) [$METRIC_LABEL_NAMESPACE]
      --metric.label.gvr=       Label for resource GroupVersionResource (default: gvr) [$METRIC_LABEL_GVR]
      --scrape.time=            Scrape time (default: 30m) [$SCRAPE_TIME]
      --config=                 Path to config file [$CONFIG]
      --cache.path=             Cache path (to folder, file://path... or azblob://storageaccount.blob.core.windows.net/containername or k8scm://{namespace}/{configmap}})
                                [$CACHE_PATH]
      --server.bind=            Server address (default: :8080) [$SERVER_BIND]
      --server.timeout.read=    Server read timeout (default: 5s) [$SERVER_TIMEOUT_READ]
      --server.timeout.write=   Server write timeout (default: 10s) [$SERVER_TIMEOUT_WRITE]

Help Options:
  -h, --help                    Show this help message
```

### Config

see [example.yaml](example.yaml)

### Authentication

Supports in-cluster authentication or via `KUBECONFIG` file.

### GOMEMLIMIT

[automemlimit](https://github.com/KimMachineGun/automemlimit) is used for automatically detecting `GOMEMLIMIT` inside containers.

| Env var            | Description                                                                                               |
|--------------------|-----------------------------------------------------------------------------------------------------------|
| `AUTOMEMLIMIT=off` | Disabling auto memlimit                                                                                   |
| `GOMEMLIMIT=0.9`   | Limits golang memory to 90% of system/cgroup memory (keep some mem available to system; default is `0.9`) |
