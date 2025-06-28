# Skaði

Skaði is a `metrics-server` sidecar reverse-proxy that enables anomaly detection and injection through the Metrics API.

## Goal

Skaði aims to extend `metrics-server` through plugins, the first of which is the anomaly detection plugin.

This plugin allows for the detection and injection of anomalies in metrics data, using the [Qdrant](https://qdrant.tech) vector database, which is used to extend the metrics API with anomaly detection capabilities.

```http request
GET /apis/metrics.k8s.io/v1beta1/pods/anomalies

{
 "kind": "PodMetricsWithAnomalyList",
 "apiVersion": "github.com/rexagod/skadi/v1alpha1",
 "metadata": {},
 "items": [
  {
   "kind": "PodMetricsWithAnomaly",
   "apiVersion": "github.com/rexagod/skadi/v1alpha1",
   "metadata": {
    "name": "coredns-674b8bbfcf-7g57n",
    "namespace": "kube-system",
    "creationTimestamp": "2025-06-27T22:11:30Z",
    "labels": {
     "k8s-app": "kube-dns",
     "pod-template-hash": "674b8bbfcf"
    }
   },
   "timestamp": "2025-06-27T22:11:20Z",
   "window": "12.813s",
   "containers": [
    {
     "name": "coredns",
     "usage": {
      "cpu": "2402559n",
      "memory": "17008Ki"
     },
     "anomaly_score": 0.0035383783
    }
   ]
  },
...
```

Note that Skaði proxies the all non-`/anomalies` requests to the `metrics-server`, to avoid breaking existing functionality.

As such, all workloads that engage with the Metrics API (or `metrics-server`, in case of Kubelet) will continue to work as expected.

## Try it out!

The [components.yaml](./assets/components.yaml) bundles `metrics-server` alongside Skaði, Qdrant, and the anomaly detection plugin.

```console
kubectl apply -f https://raw.githubusercontent.com/rexagod/skadi/refs/heads/main/assets/components.yaml
```
