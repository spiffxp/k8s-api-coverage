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
	"flag"
	"log"
	"os"
	"path"
	"strings"

	"knative.dev/test-infra/shared/prow"
	"sigs.k8s.io/k8s-api-coverage/pkg/common"
	"sigs.k8s.io/k8s-api-coverage/pkg/coveragecalculator"
	"sigs.k8s.io/k8s-api-coverage/pkg/kube"
	"sigs.k8s.io/k8s-api-coverage/pkg/tools"
)

var (
	buildFailedFlag = flag.Bool("build_failed", false, "Flag indicating if the apicoverage build failed (default: false)")
	webhookURIFlag  = flag.String("webhook-uri", "", "uri of apicoverage-webhook service, auto-detected if empty (default: \"\") ")
)

// Helper method to produce failed coverage results.
func getFailedResourceCoverages() coveragecalculator.CoveragePercentages {
	percentCoverages := make(map[string]float64)
	for resourceKind := range common.ResourceMap {
		percentCoverages[resourceKind.Kind] = 0.0
	}
	percentCoverages["Overall"] = 0.0
	return coveragecalculator.CoveragePercentages{
		ResourceCoverages: percentCoverages,
	}
}

func main() {
	flag.Parse()
	// Ensure artifactsDir exist, in case not invoked from this script
	artifactsDir := prow.GetLocalArtifactsDir()
	if _, err := os.Stat(artifactsDir); os.IsNotExist(err) {
		if err = os.MkdirAll(artifactsDir, 0777); err != nil {
			log.Fatalf("Failed to create directory: %v", err)
		}
	}
	tools.CleanupJunitFiles(artifactsDir)

	if *buildFailedFlag {
		log.Printf("Build failed, writing failed resource coverages")
		outputPath := path.Join(artifactsDir, "junit_bazel.xml")
		coverage := getFailedResourceCoverages()
		err := tools.WriteResourcePercentages(outputPath, coverage)
		if err != nil {
			log.Fatalf("Failed writing resource coverage percentages: %v", err)
		}
		log.Printf("Wrote failed resource coverage percentages to %s", outputPath)
		return
	}

	webhookURI := getWebhookURI()
	log.Printf("Using webhook-uri %s", webhookURI)

	for gvk := range common.ResourceMap {
		outputPath := path.Join(artifactsDir, strings.ToLower(gvk.Group)+"_"+strings.ToLower(gvk.Version)+"_"+strings.ToLower(gvk.Kind)+".html")
		err := tools.GetAndWriteResourceCoverage(webhookURI, gvk, outputPath)
		if err != nil {
			log.Printf("Failed retrieving resource coverage for resource %v: %v ", gvk, err)
		} else {
			log.Printf("Wrote resource coverage for resource %v to %s", gvk, outputPath)
		}
	}

	outputPath := path.Join(artifactsDir, "totalcoverage.html")
	err := tools.GetAndWriteTotalCoverage(webhookURI, outputPath)
	if err != nil {
		log.Fatalf("total coverage retrieval failed: %v", err)
	}
	log.Printf("Wrote resource coverage percentages to %s", outputPath)

	outputPath = path.Join(artifactsDir, "junit_bazel.xml")
	coverage, err := tools.GetResourcePercentages(webhookURI)
	if err != nil {
		log.Fatalf("Failed retrieving resource coverage percentages: %v", err)
	}
	err = tools.WriteResourcePercentages(outputPath, coverage)
	if err != nil {
		log.Fatalf("Failed writing resource coverage percentages: %v", err)
	}
	log.Printf("Wrote resource coverage percentages to %s", outputPath)
}

func getWebhookURI() string {
	if *webhookURIFlag != "" {
		return *webhookURIFlag
	}

	log.Printf("Autodetecting webhook-uri from service %s/%s", common.WebhookNamespace, common.CommonComponentName)

	kubeClient, err := kube.BuildKubeClient()
	if err != nil {
		log.Fatalf("Failed to get client set: %v", err)
	}
	log.Printf("Built kubernetes client")

	webhookURI, err := tools.GetWebhookServiceURI(kubeClient, common.WebhookNamespace, common.CommonComponentName)
	if err != nil {
		log.Fatalf("Error retrieving Service IP: %v", err)
	}

	return webhookURI
}
