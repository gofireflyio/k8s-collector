<p align="center">
  <img src="https://github.com/gofireflyio/k8s-collector/blob/main/project-logo.png?raw=true" alt="Firefly's image"/>
</p>

**Firefly Kubernetes Collector**

<!-- vim-markdown-toc GFM -->

* [Overview](#overview)
* [Quick Start](#quick-start)
* [Configuration](#configuration)
* [License](#license)

<!-- vim-markdown-toc -->

## Overview

This repository contains Firefly's Kubernetes Collector, which collects
information from a customer's Kubernetes cluster and sends it to the Firefly
SaaS. This means it is an on-premises component.

The collector is implemented in the [Go programming language](https://golang.org/) and packaged as an
[OCI image](https://github.com/opencontainers/image-spec). It uses the official [Go client](https://github.com/kubernetes/client-go) provided by the
Kubernetes project for the benefits it provides over manually accessing the
Kubernetes API.

The collector is currently implemented as a job meant to be run as a Kubernetes
[CronJob](https://kubernetes.io/docs/tasks/job/automated-tasks-with-cron-jobs/). While this means the job's execution interval is at the discretion
of the customer, this provides the ability to trigger the job manually at any
given time without having to restart or add triggering capabilities to a
Kubernetes [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/).

The collector collects various objects from the Kubernetes cluster and sends them
as-is to Firefly. There is a default list of resource types the collector
fetches, to which more types can be added (or removed) via configuration.

## Quick Start

Firefly's Kubernetes Collector requires:

- [Kubernetes](https://kubernetes.io/) v1.15+
- [Helm](https://helm.sh/) v3.5.0+

/ To start using the collector, follow these simple steps:

1. Use the Kubernetes Integration wizard in the Firefly dashboard to create
   an access keypair for a Kubernetes Cluster.
2. Install the collector on the cluster using [Helm](https://helm.sh/), with the
   data returned from the wizard:

   ```sh
   helm repo add firefly https://gofireflyio.github.io/k8s-collector
   helm install firefly firefly/firefly-k8s-collector \
       --set accessKey=<access_key> \
       --set secretKey=<secret_key> \
       --set clusterId=<cluster_id>
   ```

The collector's OCI-compliant Docker image is [hosted in Docker Hub](https://hub.docker.com/r/infralightio/k8s-collector). The image is
built from a [Dockerfile](Dockerfile) that uses an Alpine-based Go image
and employs a [multi-stage build](https://docs.docker.com/develop/develop-images/multistage-build/) process to compile the collector into a
[statically-linked binary](https://en.wikipedia.org/wiki/Static_library). The resulting image does not use any base layer,
thus keeping its size as small as possible and improving security.

The image is named `gofireflyio/k8s-collector`.

## Configuration

Please review the [charts/chart/values.yaml](values.yaml) file for a list of
configuration options that can be modified when installing the Helm Chart.
You may wish to modify the "schedule" setting, which controls the schedule for
the collector's execution. By default, the collector is executed once every 15
minutes. This can be changed with a [cron-compatible string](https://cron.help/).

When following the steps in the [Quick Start](#quick-start) section above, the wizard will
instruct you to assign a cluster ID for the installation. This is necessary
because Kubernetes does not provide a way to access a unique name or ID for a
cluster, a cluster identifier must be provided to the collector.

The chart provides this cluster ID to the collector via the `CLUSTER_ID` environment
variable. The cluster ID must only contain lowercase alphanumeric characters,
dashes and underscore (spaces are not allowed).

The collector must also be configured with an Firefly-provided access and secret
keys in order to be able to send data to Firefly. These keys are stored by the
chart as Kubernetes Secrets, and provided to the collector via the
`INFRALIGHT_ACCESS_KEY` and `INFRALIGHT_SECRET_KEY` environment variables,
respectively.

The collector's behavior may also be configured and modified via an optional
Kubernetes [ConfigMap](https://kubernetes.io/docs/concepts/configuration/configmap/). The complete list of configuration options
supported are not exposed via the chart's values file, but the resulting ConfigMap
can be manually modified, if necessary.

The list of resource types that are collected by the collector can be viewed
in the `DefaultResourceTypes` variable in [collector/config/config.go](collector/config/config.go).
The collector will also collect custom resources (CRDs), assuming it is provided
permission to do so. You can remove or add resource types to the list by
providing the `addTypes` and `removeTypes` values, which accept lists:

```sh
helm install firefly firefly/firefly-k8s-collector \
    --set accessKey=<access_key> \
    --set secretKey=<secret_key> \
    --set clusterId=<cluster_id> \
    --set "addTypes={secrets,applications}" \
    --set "removeTypes={configmaps}"
```

## License

This project is distributed under the terms of the [Apache License 2.0](LICENSE).
