def deploy_cert_manager():
    version = "v1.14.4"
    cert_manager_uri = "https://github.com/cert-manager/cert-manager/releases/download/{}/cert-manager.crds.yaml".format(version)
    cmd = "curl -sSL {} | kubectl apply -f -".format(cert_manager_uri)
    local(cmd, quiet=False)

def deploy_capi_crd():
    version = "v1.7.0"
    capi_uri = "https://github.com/kubernetes-sigs/cluster-api/releases/download/{}/cluster-api-components.yaml".format(version)
    cmd = "curl -sSL {} | kubectl apply -f -".format(capi_uri)
    local(cmd, quiet=False)

def deploy_bmo_crd():
    version = "v0.6.0"
    m3_uri = "https://raw.githubusercontent.com/metal3-io/baremetal-operator/v0.6.0/config/base/crds/bases/metal3.io_baremetalhosts.yaml".format(version)
    cmd = "curl -sSL {} | kubectl apply -f -".format(m3_uri)
    local(cmd, quiet=False)

def deploy_metal_crd():
    version = "da2a8154c95f3e087c3dbd798b1ff28328266dab"
    bmcSecret_uri = "https://raw.githubusercontent.com/ironcore-dev/metal-operator/{}/config/crd/bases/metal.ironcore.dev_bmcsecrets.yaml".format(version)
    cmd_bmcSecret = "curl -sSL {} | kubectl apply -f -".format(bmcSecret_uri)
    bmc_uri = "https://raw.githubusercontent.com/ironcore-dev/metal-operator/{}/config/crd/bases/metal.ironcore.dev_bmcs.yaml".format(version)
    cmd_bmc = "curl -sSL {} | kubectl apply -f -".format(bmc_uri)
    server_uri = "https://raw.githubusercontent.com/ironcore-dev/metal-operator/{}/config/crd/bases/metal.ironcore.dev_servers.yaml".format(version)
    cmd = "curl -sSL {} | kubectl apply -f -".format(server_uri)
    local(cmd_bmcSecret, quiet=False)
    local(cmd_bmc, quiet=False)
    local(cmd, quiet=False)

deploy_cert_manager()
# deploy_capi_crd()
deploy_bmo_crd()
deploy_metal_crd()

k8s_yaml('hack/cluster-api-components.yaml')
k8s_yaml('hack/cluster.yaml')
k8s_yaml('dist/install.yaml')
