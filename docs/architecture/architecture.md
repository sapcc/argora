# Argora Operator

## Overview
The Argora Operator is a Kubernetes operator handling Metal API resources based of NetBox entities as source of truth. It extends Kubernetes and utilizes its capabilities for automation and lifecycle management. This documentation covers the architecture and functionality of the operator, including the **Irconcore**, **Metal3**, and **Update** controllers.

![Argora Overview](argora.svg)

---

## Components

### 1. Irconcore Controller
The **Irconcore** controller is responsible for managing [Metal API](https://github.com/ironcore-dev/metal-operator) resources (`BMC` and `BMCSecret`) directly from Netbox based on some selection criterea defined in the ClusterImport CR. It ensures that the desired state of the cluster is maintained by:
- Reconciling ClusterImport CRs and fetching data for the cluster slection from NetBox.
- Creates/updates `BMC` and `BMCSecret` based on the selection critereas in the configuration.

#### Key Features:
- Maintains BMC based on ClusterImport CRs and fetching data from NetBox.

---

### 2. Metal3 Controller
The **Metal3** controller focuses on bare-metal provisioning and management based of a Cluster API Resources on the Kubernetes cluster (Cluster CRs). It integrates with the [Metal3](https://github.com/metal3-io/baremetal-operator) project to provide:
- Bare-metal host discovery and registration using `BareMetalHost` and a `Secret`
- Lifecycle management for bare-metal resources handled by Metal3 operators.

#### Key Features:
- Bare-metal host management.
- Integration with Metal3 APIs.

---

### 3. Update Controller
The **Update** controller handles updates of Netbox entities with a preconfigured rules for our needs. It reconciles Update CRs which have selection critereas for Netbox and uses Netbox REST API to obtain and update. For each device in the selected cluster, it updates:
- Remoteboard interface name, e.g. renaming different manufacture specific name to `remoteboard`
- Update general settings, e.g. OOB IP
- Removes unneeded VMK interfaces and IPs

#### Key Features:
- Automation on maintaining specific configuration of Netbox entities for our needs.

---

## Architecture
The operator follows a controller-based architecture, where each controller is responsible for a specific domain. These controllers interact with the Kubernetes API server to monitor and reconcile resources.

### Workflow:
1. **Resource Monitoring**: ...
2. **Reconciliation**: ...
3. **Automation**: ...
