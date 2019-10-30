/*
Copyright 2018 The Knative Authors

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

package webhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"

	"go.uber.org/zap"
	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/k8s-api-coverage/pkg/coveragecalculator"
	"sigs.k8s.io/k8s-api-coverage/pkg/resourcetree"
	"sigs.k8s.io/k8s-api-coverage/pkg/view"
)

var (
	decoder = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

const (
	// ResourceQueryParam query param name to provide the resource.
	ResourceQueryParam = "resource"

	// ResourceCoverageEndPoint is the endpoint for Resource Coverage API
	ResourceCoverageEndPoint = "/resourcecoverage"

	// TotalCoverageEndPoint is the endpoint for Total Coverage API
	TotalCoverageEndPoint = "/totalcoverage"

	// ResourcePercentageCoverageEndPoint is the end point for Resource Percentage
	// coverages API
	ResourcePercentageCoverageEndPoint = "/resourcepercentagecoverage"

	// resourceChannelQueueSize size of the queue maintained for resource channel.
	resourceChannelQueueSize = 10
)

type resourceChannelMsg struct {
	resourceGVK      schema.GroupVersionKind
	rawResourceValue []byte
}

// APICoverageRecorder type contains resource tree to record API coverage for resources.
type APICoverageRecorder struct {
	Logger         *zap.SugaredLogger
	ResourceForest resourcetree.ResourceForest
	ResourceMap    map[schema.GroupVersionKind]reflect.Type
	NodeRules      resourcetree.NodeRules
	FieldRules     resourcetree.FieldRules
	DisplayRules   view.DisplayRules

	resourceChannel chan resourceChannelMsg
	ignoredFields   coveragecalculator.IgnoredFields
}

// Init initializes the resources trees for set resources.
func (a *APICoverageRecorder) Init() {
	a.Logger.Info("APICoverageRecorder.Init")

	for resourceKind, resourceType := range a.ResourceMap {
		a.ResourceForest.AddResourceTree(resourceKind.Kind, resourceType)
	}

	ignoredFieldsFilePath := os.Getenv("KO_DATA_PATH") + "/ignoredfields.yaml"
	err := a.ignoredFields.ReadFromFile(ignoredFieldsFilePath)
	if err != nil {
		a.Logger.Errorf("Error reading file %s: %v", ignoredFieldsFilePath, err)
	}

	a.resourceChannel = make(chan resourceChannelMsg, resourceChannelQueueSize)

	go a.updateResourceCoverageTree()
}

// updateResourceCoverageTree updates the resource coverage tree.
func (a *APICoverageRecorder) updateResourceCoverageTree() {
	a.Logger.Info("APICoverageRecorder.updateResourceCoverageTree")
	for {
		channelMsg := <-a.resourceChannel
		a.Logger.Info("APICoverageRecorder.updateResourceCoverageTree received message")
		resourceType := a.ResourceMap[channelMsg.resourceGVK]
		resource := reflect.New(resourceType).Interface()
		if err := json.Unmarshal(channelMsg.rawResourceValue, resource); err != nil {
			a.Logger.Errorf("Failed unmarshalling review.Request.Object.Raw for type: %s Error: %v", channelMsg.resourceGVK.Kind, err)
			continue
		}
		resourceTree := a.ResourceForest.TopLevelTrees[channelMsg.resourceGVK.Kind]
		resourceTree.UpdateCoverage(reflect.ValueOf(resource).Elem())
		a.Logger.Info("Successfully recorded coverage for resource ", channelMsg.resourceGVK.Kind)
	}
}

// RecordResourceCoverage updates the resource tree with the request.
func (a *APICoverageRecorder) RecordResourceCoverage(w http.ResponseWriter, r *http.Request) {
	a.Logger.Info("APICoverageRecorder.RecordResourceCoverage")

	review := &v1beta1.AdmissionReview{}
	err := a.jsonRead(r, review, "review")
	if err != nil {
		a.appendAndWriteAdmissionResponse(review, false, "Admission Denied", w)
		return
	}

	gvk := schema.GroupVersionKind{
		Group:   review.Request.Kind.Group,
		Version: review.Request.Kind.Version,
		Kind:    review.Request.Kind.Kind,
	}
	op := review.Request.Operation
	raw := review.Request.Object.Raw

	// We only care about resources the repo has setup.
	if _, ok := a.ResourceMap[gvk]; !ok {
		a.Logger.Info("By-passing resource coverage update for resource : %s", gvk.Kind)
		a.appendAndWriteAdmissionResponse(review, true, "Welcome Aboard", w)
		return
	}
	a.Logger.Infof("APICoverageRecorder.RecordResourceCoverage sending to channel gvk %v, op %v, raw %s", gvk, op, string(raw))

	a.resourceChannel <- resourceChannelMsg{
		resourceGVK:      gvk,
		rawResourceValue: raw,
	}
	a.appendAndWriteAdmissionResponse(review, true, "Welcome Aboard", w)
}

// TODO(spiffxp): do we have to keep the request on the review object?
func (a *APICoverageRecorder) appendAndWriteAdmissionResponse(review *v1beta1.AdmissionReview, allowed bool, message string, w http.ResponseWriter) {
	review.Response = &v1beta1.AdmissionResponse{
		Allowed: allowed,
		Result: &v1.Status{
			Message: message,
		},
	}
	a.jsonWrite(w, review, "review response")
}

// getCoverage returns the CoverageValues and TypeCoverage for a given kind
func (a *APICoverageRecorder) getCoverage(kind string) (coveragecalculator.CoverageValues, []coveragecalculator.TypeCoverage) {
	tree := a.ResourceForest.TopLevelTrees[kind]
	typeCoverage := tree.BuildCoverageData(a.NodeRules, a.FieldRules, a.ignoredFields)
	coverageValues := coveragecalculator.CalculateTypeCoverage(typeCoverage)
	return coverageValues, typeCoverage
}

// GetResourceCoverage retrieves resource coverage data for the passed in resource via query param.
func (a *APICoverageRecorder) GetResourceCoverage(w http.ResponseWriter, r *http.Request) {
	a.Logger.Infof("APICoverageRecorder.GetResourceCoverage")

	resource := r.URL.Query().Get(ResourceQueryParam)
	if _, ok := a.ResourceForest.TopLevelTrees[resource]; !ok {
		fmt.Fprintf(w, "Resource information not found for resource: %s", resource)
		return
	}

	coverageValues, typeCoverage := a.getCoverage(resource)

	if htmlData, err := view.GetHTMLDisplay(typeCoverage, coverageValues); err != nil {
		fmt.Fprintf(w, "Error generating html file %v", err)
	} else {
		fmt.Fprint(w, htmlData)
	}
}

// GetTotalCoverage goes over all the resources setup for the apicoverage tool and returns total coverage values.
func (a *APICoverageRecorder) GetTotalCoverage(w http.ResponseWriter, r *http.Request) {
	a.Logger.Infof("APICoverageRecorder.GetTotalCoverage")

	totalCoverage := coveragecalculator.CoverageValues{}
	for resource := range a.ResourceMap {
		coverageValues, _ := a.getCoverage(resource.Kind)
		totalCoverage.Accumulate(coverageValues)
	}

	a.jsonWrite(w, totalCoverage, "total coverage")
}

// GetResourceCoveragePercentages goes over all the resources setup for the
// apicoverage tool and returns percentage coverage for each resource.
func (a *APICoverageRecorder) GetResourceCoveragePercentages(w http.ResponseWriter, r *http.Request) {
	a.Logger.Infof("APICoverageRecorder.GetResourceCoveragePercentages")

	percentCoverages := make(map[string]float64)
	totalCoverage := coveragecalculator.CoverageValues{}
	for resource := range a.ResourceMap {
		coverageValues, _ := a.getCoverage(resource.Kind)
		percentCoverages[resource.Kind] = coverageValues.PercentCoverage
		totalCoverage.Accumulate(coverageValues)
	}
	percentCoverages["Overall"] = totalCoverage.PercentCoverage

	a.jsonWrite(w, coveragecalculator.CoveragePercentages{ResourceCoverages: percentCoverages}, "percent coverage")
}

func (a *APICoverageRecorder) jsonRead(r *http.Request, obj runtime.Object, description string) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s := fmt.Sprintf("error reading %s request: %v", description, err)
		a.Logger.Errorf(s)
		return fmt.Errorf(s)
	}
	_, _, err = decoder.Decode(body, nil, obj)
	if err != nil {
		s := fmt.Sprintf("Unable to decode %s request: %v", description, err)
		a.Logger.Errorf(s)
		return fmt.Errorf(s)
	}
	return nil
}

func (a *APICoverageRecorder) jsonWrite(w http.ResponseWriter, v interface{}, description string) {
	body, err := json.Marshal(v)
	if err != nil {
		s := fmt.Sprintf("error marshalling %s response: %v", description, err)
		fmt.Fprintf(w, s)
		a.Logger.Error(s)
		return
	}
	_, err = w.Write(body)
	if err != nil {
		s := fmt.Sprintf("error writing %s response: %v", description, err)
		fmt.Fprintf(w, s)
		a.Logger.Error(s)
	}
}
