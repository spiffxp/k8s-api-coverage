/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"container/list"
	"log"
	"net/http"
	"net/http/pprof"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/signals"
	"sigs.k8s.io/k8s-api-coverage/pkg/common"
	"sigs.k8s.io/k8s-api-coverage/pkg/resourcetree"
	"sigs.k8s.io/k8s-api-coverage/pkg/rules"
	"sigs.k8s.io/k8s-api-coverage/pkg/webhook"
)

// TODO(spiffxp): I don't think GVK is plumbed all the way through, and the words
// resource and kind are used interchangeably here, where I notice they mean subtly
// different things in apimachinery docs

// TODO(spiffxp): the total number of fields to cover grows as the cluster is
// exercised; why isn't resourcetree seeing all of these fields to begin with?
// BECAUSE they are pointers, and it won't traverse nil pointers

// TODO(spiffxp): if the container dies, the pod will crashloopbackoff because this
// refuses to come back up because it fails on webhook registration; I think this
// should either ignore, ignore if hook ownerref is the same as what this would set,
// or delete/recreate. Either way, coverage state isn't preserved across restarts,
// does it make sense to try and persist?

// TODO(spiffxp): Admission webhooks don't get access to user agent. It's not
// clear to me how else to plumb through which test is exercising a given
// resource/field. There is userInfo but that would require a different user
// for each test I think. The other alternative is to rewrite this to receive
// audit log webhooks, although they are still in alpha so not dynamically
// configurable by default

// TODO(spiffxp): I don't think subresources are getting covered, eg: I'm tailing logs but
// that option isn't showing up in artifacts/_v1_podlogoptions.html

// TODO(spiffxp): fatal error: concurrent map iteration and map write
// soooo I guess I can't update coverage artifacts locally as tests are going on
/*
goroutine 8701 [running]:
runtime.throw(0x13098f3, 0x26)
	/usr/local/go/src/runtime/panic.go:617 +0x72 fp=0xc00011f2c0 sp=0xc00011f290 pc=0x42dc72
runtime.mapiternext(0xc00011f380)
	/usr/local/go/src/runtime/map.go:860 +0x597 fp=0xc00011f348 sp=0xc00011f2c0 pc=0x4107f7
k8s.io/apimachinery/pkg/util/sets.String.Union(0xc0014580c0, 0xc000ea3e30, 0x3)
	/go/pkg/mod/k8s.io/apimachinery@v0.0.0-20190817020851-f2f3a405f61d/pkg/util/sets/string.go:115 +0x190 fp=0xc00011f3f0 sp=0xc00011f348 pc=0x798090
sigs.k8s.io/k8s-api-coverage/pkg/coveragecalculator.(*FieldCoverage).Merge(...)
	/go/src/app/pkg/coveragecalculator/coveragedata.go:37
sigs.k8s.io/k8s-api-coverage/pkg/resourcetree.(*ResourceForest).getConnectedNodeCoverage(0xc0003cbe08, 0x14fc3e0, 0x12b3940, 0x1ed89c0, 0x1, 0x1, 0xc001424a50, 0x0, 0x0, 0x0, ...)
	/go/src/app/pkg/resourcetree/resourceforest.go:74 +0x3c1 fp=0xc00011f620 sp=0xc00011f3f0 pc=0xb5b0b1
sigs.k8s.io/k8s-api-coverage/pkg/resourcetree.(*StructKindNode).buildCoverageData(0xc000800420, 0xc00148e9a0, 0x1ed89b8, 0x1, 0x1, 0x1ed89c0, 0x1, 0x1, 0xc001424a50, 0xc001462660)
	/go/src/app/pkg/resourcetree/structkindnode.go:91 +0xaa fp=0xc00011f838 sp=0xc00011f620 pc=0xb5c6ca
sigs.k8s.io/k8s-api-coverage/pkg/resourcetree.(*StructKindNode).buildCoverageData(0xc000800360, 0xc00148e9a0, 0x1ed89b8, 0x1, 0x1, 0x1ed89c0, 0x1, 0x1, 0xc001424a50, 0xc001462660)
	/go/src/app/pkg/resourcetree/structkindnode.go:101 +0x51a fp=0xc00011fa50 sp=0xc00011f838 pc=0xb5cb3a
sigs.k8s.io/k8s-api-coverage/pkg/resourcetree.(*ResourceTree).BuildCoverageData(0xc00011fbf0, 0x1ed89b8, 0x1, 0x1, 0x1ed89c0, 0x1, 0x1, 0xc001424a50, 0xc001462630, 0x1, ...)
	/go/src/app/pkg/resourcetree/resourcetree.go:95 +0x188 fp=0xc00011fb00 sp=0xc00011fa50 pc=0xb5baa8
sigs.k8s.io/k8s-api-coverage/pkg/webhook.(*APICoverageRecorder).GetResourceCoveragePercentages(0xc0003cbe00, 0x14c1400, 0xc0016a6050, 0xc000f0c500)
	/go/src/app/pkg/webhook/apicoverage_recorder.go:247 +0x33a fp=0xc00011fcb8 sp=0xc00011fb00 pc=0x109f9aa
sigs.k8s.io/k8s-api-coverage/pkg/webhook.(*APICoverageRecorder).GetResourceCoveragePercentages-fm(0x14c1400, 0xc0016a6050, 0xc000f0c500)
	/go/src/app/pkg/webhook/apicoverage_recorder.go:228 +0x48 fp=0xc00011fce8 sp=0xc00011fcb8 pc=0x10a3238

goroutine 6 [runnable]:
strconv.frexp10Many(0xc001a39858, 0xc001a398a0, 0xc001a39840, 0x1929e)
	/usr/local/go/src/strconv/extfloat.go:349 +0xea
strconv.(*extFloat).ShortestDecimal(0xc001a398a0, 0xc001a398f0, 0xc001a39858, 0xc001a39840, 0x1e91090)
	/usr/local/go/src/strconv/extfloat.go:556 +0x21b
...
go.uber.org/zap.(*SugaredLogger).Info(...)
	/go/pkg/mod/go.uber.org/zap@v1.12.0/sugar.go:102
sigs.k8s.io/k8s-api-coverage/pkg/webhook.(*APICoverageRecorder).updateResourceCoverageTree(0xc0003cbe00)
	/go/src/app/pkg/webhook/apicoverage_recorder.go:100 +0x550
created by sigs.k8s.io/k8s-api-coverage/pkg/webhook.(*APICoverageRecorder).Init
	/go/src/app/pkg/webhook/apicoverage_recorder.go:83 +0x311

*/

// main builds the necessary webhook configuration, HTTPServer and starts the webhook.
func main() {
	namespace := common.WebhookNamespace
	if len(namespace) == 0 {
		log.Fatal("Namespace value to used by the webhook is not set")
	}

	webhookConf := webhook.BuildWebhookConfiguration(common.CommonComponentName, common.WebhookNamespace)
	recorder := webhook.APICoverageRecorder{
		Logger: webhookConf.Logger,
		ResourceForest: resourcetree.ResourceForest{
			Version:        "v1alpha1",
			ConnectedNodes: make(map[string]*list.List),
			TopLevelTrees:  make(map[string]resourcetree.ResourceTree),
		},
		ResourceMap:  common.ResourceMap,
		NodeRules:    rules.NodeRules,
		FieldRules:   rules.FieldRules,
		DisplayRules: rules.GetDisplayRules(),
	}
	recorder.Init()

	mux := http.NewServeMux()
	mux.HandleFunc("/", recorder.RecordResourceCoverage)
	mux.HandleFunc(webhook.ResourceCoverageEndPoint, recorder.GetResourceCoverage)
	mux.HandleFunc(webhook.TotalCoverageEndPoint, recorder.GetTotalCoverage)
	mux.HandleFunc(webhook.ResourcePercentageCoverageEndPoint, recorder.GetResourceCoveragePercentages)

	// TODO(spiffxp): expose on its own mux like prow does?
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	resources := []schema.GroupVersionKind{}
	for gvk := range recorder.ResourceMap {
		resources = append(resources, gvk)
	}
	log.Printf("Passing in resources %+v", resources)
	err := webhookConf.Run(mux, resources, namespace, signals.SetupSignalHandler())
	if err != nil {
		log.Fatalf("Encountered error setting up Webhook: %v", err)
	}
}
