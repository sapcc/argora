<!--
SPDX-FileCopyrightText: 2025 SAP SE

SPDX-License-Identifier: Apache-2.0
-->

# Argora
Contains controllers that provide custom Kubernetes resources for Metal3 and IronCore metal operator using Netbox as the source of truth.

## Description

Argora is a Kubernetes operator designed to manage Metal3 and IronCore resources using Netbox as the authoritative source of truth. It simplifies the process of provisioning and managing bare metal servers by integrating with Netbox for inventory and configuration management. The operator ensures that the desired state of the infrastructure is maintained by continuously reconciling the actual state with the declared state in Kubernetes custom resources.

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster

#### Using Tilt

Helm is used to Template manifest of the operators. You need to provide `helm` values under `dist/chart/values.yaml` in the following format:

```yaml
...

config:
  serverController: "ironcore"
  ironCore:
    name: ""
    region: "qa-de-1"
    types: "cc-kvm-compute,cc-kvm-admin"
  netboxURL: "https://netbox.global.cloud.sap/"

credentials:
  bmcUser: "<user>"
  bmcPassword: "<password>"
  netboxToken: "<token>"
```

**Run on dev cluster**

Install `kind` if needed:

```bash
kind create cluster -n kind
```

Start `tilt`:

```bash
make tilt
```

#### Using various Kubernetes cluster

**Build and push your image to the location specified by `IMG_REPO` and `IMG_TAG` environment variables:**

```sh
export IMG_REPO=<some-registry>/argora
export IMG_TAG=<tag>
make docker-build docker-push
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/argora:<tag>
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make helm-build
```

**NOTE:** The makefile target mentioned above generates an 'manifest.yaml'
file in the dist directory. This file contains all the resources built
with helm, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f ./dist/manifest.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing

**NOTE:** Run `make help` and `make help-ext` for more information on all potential `make` targets

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

