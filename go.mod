module sigs.k8s.io/k8s-api-coverage

go 1.13

// Synced from knative/pkg.
// The build fails against 0.12.6 and newer because
// stackdriver.Options.GetMonitoredResource was removed.
replace contrib.go.opencensus.io/exporter/stackdriver => contrib.go.opencensus.io/exporter/stackdriver v0.12.5 // indirect

// for repo in api apimachinery client-go; do
//   export sha=$(hub api /repos/kubernetes/$repo/tags?per_page=100 | jq -r 'map(select(.name=="kubernetes-1.16.2"))[].commit.sha' | cut -c1-7)
//   go get k8s.io/$repo@$sha
// done
replace (
	k8s.io/api => k8s.io/api v0.0.0-20190819141258-3544db3b9e44
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190819141724-e14f31a72a77
)

// [[override]]
// name = "k8s.io/api"
// version = "kuberetes-1.15.3"
//
// [[override]]
// name = "k8s.io/apimachinery"
// version = "kubernetes-1.15.3"
//
// [[override]]
// name = "k8s.io/code-generator"
// version = "kubernetes-1.15.3"
//
// [[override]]
// name = "k8s.io/apiextensions-apiserver"
// version = "kubernetes-1.15.3"
//
// [[override]]
// name = "k8s.io/client-go"
// version = "kubernetes-1.15.3"
//
// [[override]]
// name = "k8s.io/apiserver"
// version = "kubernetes-1.15.3"
//
// [[override]]
// name = "k8s.io/metrics"
// version = "kubernetes-1.15.3"
//
// [[override]]
// name = "k8s.io/kube-openapi"
// # This is the version at which k8s.io/apiserver depends on this at its 1.15.3 tag.
// revision = "b3a7cee44a305be0a69e1b9ac03018307287e1b0"
//
// [[override]]
// name = "sigs.k8s.io/structured-merge-diff"
// # This is the version at which k8s.io/apiserver depends on this at its 1.15.3 tag.
// revision = "e85c7b244fd2cc57bb829d73a061f93a441e63ce"

// [[constraint]]
//  name = "github.com/google/go-containerregistry"
//  # HEAD as of 2019-09-10
//  revision = "b02d448a3705facf11018efff34f1d2830be5724"

// [[constraint]]
//  name = "go.opencensus.io"
//  version = "0.22.0"

// [[override]]
//  name = "go.uber.org/zap"
//  revision = "67bc79d13d155c02fd008f721863ff8cc5f30659"

// [[override]]
//  name = "k8s.io/api"
//  version = "kuberetes-1.15.3"

require (
	contrib.go.opencensus.io/exporter/prometheus v0.1.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.0.0-00010101000000-000000000000 // indirect
	github.com/google/go-containerregistry v0.0.0-20191029173801-50b26ee28691 // indirect
	github.com/markbates/inflect v1.0.4
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a // indirect
	github.com/pkg/errors v0.8.1
	go.opencensus.io v0.22.1 // indirect
	go.uber.org/zap v1.12.0
	google.golang.org/api v0.13.0 // indirect
	google.golang.org/genproto v0.0.0-20191028173616-919d9bdd9fe6 // indirect
	gopkg.in/yaml.v2 v2.2.4
	k8s.io/api v0.0.0-20191016110408-35e52d86657a
	k8s.io/apimachinery v0.0.0-20191004115801-a2eda9f80ab8
	k8s.io/client-go v0.0.0-20191016111102-bec269661e48
	knative.dev/pkg v0.0.0-20191030060811-3732de580201
	knative.dev/serving v0.10.0
	knative.dev/test-infra v0.0.0-20191030013311-34a629e61afc
)
