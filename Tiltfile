config.define_string("NETBOX_URL")
config.define_string("BININFO_BUILD_DATE")
config.define_string("BININFO_VERSION")
config.define_string("BININFO_COMMIT_HASH")
config.define_string("TARGET")

cfg = config.parse()
target = cfg.get("TARGET", "manager")
netbox_url = cfg.get("NETBOX_URL", "")

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

def deploy_onboarding_crd():
    version = "c91ba0d76b78ab8d24a58a41b31bcfab6ff4a995"
    metalcluster_uri = "https://raw.githubusercontent.com/SAP-cloud-infrastructure/metal-onboarding-operator/{}/config/crd/bases/onboarding.metal.ironcore.dev_metalclusters.yaml".format(version)
    local("curl -sSL {} | kubectl apply -f -".format(metalcluster_uri), quiet=False)

def deploy_metaldhcp_crd():
    version = "07bd42b2f6975f95a14b7e1bda03906988bf83c3"
    subnet_uri = "https://raw.githubusercontent.com/SAP-cloud-infrastructure/metaldhcp/{}/config/crd/bases/dhcp.metal.ironcore.dev_subnets.yaml".format(version)
    local("curl -sSL {} | kubectl apply -f -".format(subnet_uri), quiet=False)


build_args = {k: v for k, v in {
    "BININFO_BUILD_DATE": cfg.get('BININFO_BUILD_DATE'),
    "BININFO_VERSION": cfg.get('BININFO_VERSION'),
    "BININFO_COMMIT_HASH": cfg.get('BININFO_COMMIT_HASH')
}.items() if v != None}

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
deploy_onboarding_crd()
deploy_metaldhcp_crd()

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

# Values: chart defaults → tracked dev overrides → local secrets (gitignored)
helm_values = ['dist/chart/values.yaml', 'hack/values.yaml']
if os.path.exists('hack/values.local.yaml'):
    helm_values.append('hack/values.local.yaml')

# NETBOX_URL from tilt_config.json overrides whatever is in the values files
helm_set = []
if netbox_url != "":
    helm_set.append('netboxURL=' + netbox_url)

k8s_yaml(helm(
    'dist/chart',
    name='argora',
    namespace='argora-system',
    values=helm_values,
    set=helm_set,
))

if target == 'debug':
    k8s_resource('argora-controller-manager', port_forwards=3000)
