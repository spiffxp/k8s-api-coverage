# k8s-api-coverage

Is it possible to extract / decouplate knative's api coverage from knative?

The tooling / code is currently spread across
- knative/test-infra
- knative/pkg
- knative/serving

# Goals

- get all the knative code related to coverage in one place
- get it to build with go.mod
- get installed into kind
- get coverage from k8s conformance tests

Nice to haves

- replace/pull-in the remaining knative util code
- redo hardcoded stuff to use flags instead a-la prow components

# Generate Reports

Tested with:
- `kind v0.6.0-alpha go1.12.3 darwin/amd64`
- a branch off of kubernetes @ 53bb8299

Terminal 1
```sh
# setup a cluster to test against
kind create cluster
KUBECONFIG="$(kind get kubeconfig-path)"
export KUBECONFIG

# build and install the apicoverage webhook and supporting resources
./hack/local.sh

# run tests in terminal 2 and wait to finish before proceeding

# open a local authed proxy to the apiserver
kubectl proxy&

# construct the webhook proxy uri
export WEBHOOK_URI=http://127.0.0.1:8001/api/v1/namespaces/k8s-api-coverage/services/https:apicoverage-webhook:443/proxy

# dump coverage reports to ./artifacts
./k8s-api-coverage-client -webhook-uri $WEBHOOK_URI
```

Terminal 2 - run tests
```sh
# run tests (this is hacked out of kind/hack/ci)
cp ./hack/run-tests.sh ~/w/kubernetes/kubernetes
cd !$
make WHAT="test/e2e/e2e.test vendor/github.com/onsi/ginkgo/ginkgo cmd/kubectl"
./run-tests.sh
```

# Sample Reports

I last ran this a few weeks ago and things have drifted since then. These
sample reports are intended to give a sense of what insights we can derive,
and whether it's worth continuing to pursue this approach

- `./sample-reports/totalcoverage.html` - this is all v1 resources
- `./sample-reports/_v1_pod.html` - podspec fields across all resources

# Uncovered or partially covered fields reachable from Pod

This doesn't include the full path to reach, as sometimes there are multiple
paths. For example, in the case of handlers, we don't distinguish whether they
are for liveness vs readiness probes

- `ConfigMapEnvSource.Optional` - I will have a PR out for this
- `ConfigMapKeySelector.Optional` - I will have a PR out for this
- `Container.TTY` - partial
  - missing `true`, is this testable?
- `Container.VolumeDevices`
- `Container.WorkingDir`
- `EmptyDirVolumeSource.Medium` - partial
  - This is missing `hugepages`, which we can't guarantee on all k8s clusters
- `EmptyDirVolumeSource.SizeLimit`
- `Handler.TCPSocket` - I have a PR out for this
- `HTTPGetAction.HTTPHeaders`
- `ObjectMeta.Initializers`
- `ObjectMeta.ManagedFields`
- `PodSecurityContext.RunAsGroup`
- `PodSecurityContext.RunAsNonRoot`
- `PodSecurityContext.SupplementalGroups`
- `PodSecurityContext.Sysctls` - suggest we ignore, can't guarantee on all k8s clusters
- `PodSecurityContext.WindowsOptions` - suggest we ignore, can't guarantee on all k8s clusters
- `PodSpec.Affinity`
- `PodSpec.DNSConfig`
- `PodSpec.DNSPolicy` - maybe partial?
  - is there more than ClusterFirst and Default?
- `PodSpec.HostIPC` - partial
  - missing `true`, is this fair to expect on all k8s clusters?
- `PodSpec.HostPID` - partial
  - missing `true`, is this fair to expect on all k8s clusters?
- `PodSpec.ImagePullSecrets`
- `PodSpec.PreemptionPolicy`
- `PodSpec.PriorityClassName`
- `PodSpec.ReadinessGates`
- `PodSpec.RuntimeClassName`
- `PodSpec.ShareProcessNamespace`
  - is this fair to expect on all k8s clusters?
- `SecretEnvSource.Optional` - I will have a PR out for this
- `SecretKeySelector.Optional` - I will have a PR out for this
- `SELinuxOptions.Role`
- `SELinuxOptions.Type`
- `SELinuxOptions.User`
- `Toleration.Effect` - partial
  - do we care to test more than `NoExecute`?
- `Toleration.Operator` - partial
  - do we care to test more than `Exists`?
- `Toleration.Value`
- `VolumeMount.MountPropagation`
- `VolumeMount.SubPathExpr`
- `VolumeProjection.ServiceAccountToken`

# Lessons Learned

- The data gathered isn't very granular compared to what we get with apisnoop.
  It's aggregrated across all requests made while the webhook is installed.
  Since it's an AdmissionWebhook, it doesn't have access to user-agent.
- This tool combines coverage across all resources. For example, PodSpec
  coverage is computed from all PodSpecs whether in a ReplicaSet, Deployment,
  Pod, DaemonSet, etc. Conversely, we cannot tell whether fields are covered
  "directly" vs. "via a rube goldberg interaction"
- The set of unconvered fields grows as resources are sent; this tool doesn't
  walk optional/default-nil resources initially.
- This tool enumerates true/false and possible-enum values, but it can't know
  whether all enum values have been covered, and misses some
  empty-string-as-enum values

Overall I would not recommend we use this extensively as-is, but it raised
enough uncovered fields to start a conversation. As a next step I want to see
if apisnoop agrees with the findings here.
