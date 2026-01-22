# Helm Installation Guide

This guide will help you install the Argora Operator using Helm.

## Prerequisites

- Kubernetes cluster (v1.16+)
- Helm (v3.0.0+)

## Steps

1. **Install the Chart**

   Install the Argora Operator chart with the default values.

   ```sh
   helm install argora-operator dist/chart
   ```

   To customize the installation, you can override the default values using a `values.yaml` file or the `--set` flag.

   ```sh
   helm install argora-operator dist/chart -f /path/to/your/values.yaml
   ```

2. **Verify the Installation**

   Check the status of the Helm release to ensure that the Argora Operator is installed successfully.

   ```sh
   helm status argora-operator
   ```

   You should see output indicating that the Argora Operator pods are running.

## Configuration

The `values.yaml` file allows you to configure various aspects of the Argora Operator. Below are some of the key configurations:

- **controllerManager**: Settings for the operator deployment.

### Controller Manager

| Key                                               | Description                                                     | Default Value                                                                                 |
|---------------------------------------------------|-----------------------------------------------------------------|-----------------------------------------------------------------------------------------------|
| `controllerManager.replicas`                      | Number of replicas for the manager deployment                   | `1`                                                                                           |
| `controllerManager.container.image.repository`    | Image repository for the manager container                      | `registry/metal-operator`                                                                     |
| `controllerManager.container.image.tag`           | Image tag for the manager container                             | `"v0.1.0"`                                                                                    |
| `controllerManager.container.args`                | Arguments for the manager container                             | `--probe-image=probe-image`, `--probe-os-image=probe-os-image`, `--registry-url=registry-url` |
| `controllerManager.container.resources`           | Resource requests and limits for the manager container          | `{cpu: 500m, memory: 128Mi}` (limits), `{cpu: 10m, memory: 64Mi}` (requests)                  |
| `controllerManager.container.livenessProbe`       | Liveness probe configuration for the manager container          | `{initialDelaySeconds: 15, periodSeconds: 20, httpGet: {path: /healthz, port: 8081}}`         |
| `controllerManager.container.readinessProbe`      | Readiness probe configuration for the manager container         | `{initialDelaySeconds: 5, periodSeconds: 10, httpGet: {path: /readyz, port: 8081}}`           |
| `controllerManager.container.securityContext`     | Security context for the manager container                      | `{allowPrivilegeEscalation: false, capabilities: {drop: ["ALL"]}}`                            |
| `controllerManager.securityContext`               | Security context for the manager pod                            | `{runAsNonRoot: true, seccompProfile: {type: RuntimeDefault}}`                                |
| `controllerManager.terminationGracePeriodSeconds` | Termination grace period for the manager pod                    | `10`                                                                                          |
| `controllerManager.serviceAccountName`            | Service account name for the manager pod                        | `metal-operator-controller-manager`                                                           |
| `controllerManager.nodeSelector`                  | Node selector for the manager pod                               | `{kubernetes.io/os: linux, kubernetes.io/arch: arm64}`                                        |
| `controllerManager.tolerations`                   | Tolerations for the manager pod                                 | `[{key: node-role.kubernetes.io/control-plane, effect: NoSchedule}]`                          |

- **credentials**: Credentials which will result in a Secret named `argora-secret`.

### Credentials

| Key                       | Description      |
|---------------------------|------------------|
| `credentials.bmcUser`     | BMC username     |
| `credentials.bmcPassword` | BMC password     |
| `credentials.netboxToken` | NetBox API token |

- **rbac**: Enable or disable RBAC.
- **crd**: Enable or disable CRDs.
- **metrics**: Enable or disable metrics export.
- **webhook**: Enable or disable webhooks.
- **prometheus**: Enable or disable Prometheus ServiceMonitor.
- **certmanager**: Enable or disable cert-manager injection.
- **networkPolicy**: Enable or disable NetworkPolicies.

### ClusterImport and Update Custom Resources

The Helm chart supports declarative creation of `ClusterImport` and `Update` custom resources through the values file. These resources enable you to import and update cluster configurations from Netbox.

#### ClusterImport

`ClusterImport` resources allow you to import cluster information from Netbox into your Kubernetes cluster. You can define multiple ClusterImport resources, each with its own set of cluster selectors.

**Values Configuration:**

```yaml
clusterImport:
  my-cluster-import:
    clusters:
      - name: "prod-cluster-01"
        region: "us-west"
        type: "compute"
      - name: "prod-cluster-02"
        region: "us-east"
        type: "storage"
  another-import:
    clusters:
      - name: "dev-cluster"
        region: "emea"
        type: "compute"
```

Each top-level key under `clusterImport` (e.g., `my-cluster-import`, `another-import`) becomes the name of a ClusterImport resource. The `clusters` field contains an array of cluster selectors with the following optional fields:

- `name`: The name of the cluster to import from Netbox
- `region`: The region where the cluster is located
- `type`: The type of cluster (e.g., `compute`, `storage`)

#### Update

`Update` resources allow you to update cluster configurations based on Netbox data. The structure is identical to ClusterImport resources.

**Values Configuration:**

```yaml
update:
  my-cluster-update:
    clusters:
      - name: "prod-cluster-01"
        region: "us-west"
        type: "compute"
      - name: "prod-cluster-02"
        region: "us-east"
        type: "storage"
  scheduled-update:
    clusters:
      - region: "emea"
        type: "compute"
```

The cluster selectors support the same fields as ClusterImport:

- `name`: The name of the cluster to update from Netbox
- `region`: The region where the cluster is located
- `type`: The type of cluster

**Note:** All fields in the cluster selectors are optional. You can use any combination of `name`, `region`, and `type` to select clusters from Netbox.

**Example Installation with ClusterImport and Update:**

```sh
helm install argora-operator dist/chart --set-string credentials.netboxToken=mytoken \
  --set-file values=custom-values.yaml
```

Where `custom-values.yaml` contains:

```yaml
credentials:
  bmcUser: "admin"
  bmcPassword: "password"
  netboxToken: "your-netbox-token"

netboxURL: "https://netbox.example.com"

clusterImport:
  initial-import:
    clusters:
      - name: "cluster1"
        region: "emea"
        type: "compute"

update:
  periodic-sync:
    clusters:
      - region: "emea"
        type: "compute"
```

Refer to the `values.yaml` file for more details on each configuration option.

## Uninstallation

To uninstall the Metal Operator, run the following command:

```sh
helm uninstall metal-operator
```

This will remove all the resources associated with the Metal Operator.

## Additional Information

For more detailed information, refer to the official documentation and Helm chart repository.
