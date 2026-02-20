config.define_string("BININFO_BUILD_DATE")
config.define_string("BININFO_VERSION")
config.define_string("BININFO_COMMIT_HASH")
config.define_string("TARGET")

cfg = config.parse()
target = cfg.get("TARGET", "manager")

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


build_args = {
    "BININFO_BUILD_DATE": cfg.get('BININFO_BUILD_DATE'),
    "BININFO_VERSION": cfg.get('BININFO_VERSION'),
    "BININFO_COMMIT_HASH": cfg.get('BININFO_COMMIT_HASH')
}

if target == 'debug':
    docker_build(
        'controller:latest',
        '.',
        target=target,
        # Forward all container args (including the first, which is $0 under sh -c) to the manager
        entrypoint='set -- "$0" "$@"; exec dlv exec /manager --headless --listen=:3000 --accept-multiclient --continue -- "$@"',
        build_args=build_args
    )
else:
    docker_build(
        'controller:latest',
        '.',
        build_args=build_args
    )

deploy_cert_manager()
# deploy_capi_crd()
deploy_bmo_crd()
deploy_metal_crd()

load('ext://namespace', 'namespace_create', 'namespace_inject')

namespace_create('capi-runtime')
namespace_create('argora-system')

# k8s_yaml('hack/deploy/cluster-api-components.yaml')

k8s_yaml('hack/crd/cluster.x-k8s.io_clusters.yaml')
k8s_yaml('hack/crd/ipam.cluster.x-k8s.io_ipaddresses.yaml')
k8s_yaml('hack/crd/ipam.cluster.x-k8s.io_ipaddressclaims.yaml')
k8s_yaml('hack/crd/ipam.cluster.x-k8s.io_globalinclusterippools.yaml')
k8s_yaml('hack/crd/metal.ironcore.dev_servers.yaml')
k8s_yaml('hack/crd/metal.ironcore.dev_serverclaims.yaml')

k8s_yaml('hack/deploy/cluster.yaml')
k8s_yaml('dist/install.yaml')

k8s_yaml('config/samples/argora_v1alpha1_update.yaml')
k8s_yaml('config/samples/argora_v1alpha1_clusterimport.yaml')

if target == 'debug':
    k8s_resource('argora-controller-manager', port_forwards=3000)