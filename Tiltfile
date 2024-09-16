config.define_string("BININFO_BUILD_DATE")
config.define_string("BININFO_VERSION")
config.define_string("BININFO_COMMIT_HASH")

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

docker_build('argora-dev', '.', build_args={"BININFO_BUILD_DATE": config.parse()['BININFO_BUILD_DATE'], "BININFO_VERSION": config.parse()['BININFO_VERSION'], "BININFO_COMMIT_HASH": config.parse()['BININFO_COMMIT_HASH']})

deploy_cert_manager()
#deploy_capi_crd()
deploy_bmo_crd()

k8s_yaml('deployments/cluster-api-components.yaml')
k8s_yaml('config/rbac/role.yaml')
k8s_yaml('deployments/argora.yaml')
k8s_yaml('deployments/test/cluster.yaml')
k8s_yaml('deployments/secret.yaml')

