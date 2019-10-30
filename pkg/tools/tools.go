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

package tools

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	"sigs.k8s.io/k8s-api-coverage/pkg/coveragecalculator"
	"sigs.k8s.io/k8s-api-coverage/pkg/view"
	"sigs.k8s.io/k8s-api-coverage/pkg/webhook"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// tools.go contains utility methods to help repos use the k8s-api-coverage tool.

const (
	// WebhookResourceCoverageEndPoint constant for resource coverage API endpoint.
	WebhookResourceCoverageEndPoint = "%s" + webhook.ResourceCoverageEndPoint + "?resource=%s"

	// WebhookTotalCoverageEndPoint constant for total coverage API endpoint.
	WebhookTotalCoverageEndPoint = "%s" + webhook.TotalCoverageEndPoint

	// WebhookResourcePercentageCoverageEndPoint constant for
	// ResourcePercentageCoverage API endpoint.
	WebhookResourcePercentageCoverageEndPoint = "%s" + webhook.ResourcePercentageCoverageEndPoint
)

var (
	jUnitFileRegexExpr = regexp.MustCompile(`junit_.*\.xml`)
)

// GetDefaultKubePath helper method to fetch kubeconfig path.
func GetDefaultKubePath() (string, error) {
	var (
		usr *user.User
		err error
	)
	if usr, err = user.Current(); err != nil {
		return "", fmt.Errorf("error retrieving current user: %v", err)
	}

	return path.Join(usr.HomeDir, ".kube/config"), nil
}

// GetWebhookServiceURI returns "https://ip:port" for the given service in the
// given namespace
func GetWebhookServiceURI(kubeClient kubernetes.Interface, namespace string, serviceName string) (string, error) {
	svc, err := kubeClient.CoreV1().Services(namespace).Get(serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Could not retrieve service %s/%s: %v", namespace, serviceName, err)
	}
	switch svc.Spec.Type {
	case corev1.ServiceTypeLoadBalancer:
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return "", fmt.Errorf("Found zero Ingress instances for service %s/%s of type %s", namespace, serviceName, svc.Spec.Type)
		}
		return "https://" + svc.Status.LoadBalancer.Ingress[0].IP + ":" + fmt.Sprint(svc.Spec.Ports[0].Port), nil
	case corev1.ServiceTypeNodePort:
		nodes, err := kubeClient.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return "", fmt.Errorf("Could not list nodes to get IP for service %s/%s of type %s: %v", namespace, serviceName, svc.Spec.Type, err)
		} else if len(nodes.Items) == 0 {
			return "", fmt.Errorf("Found zero nodes to get IP for service %s/%s of type %s", namespace, serviceName, svc.Spec.Type)
		}
		node := nodes.Items[0]
		if len(node.Status.Addresses) == 0 {
			return "", fmt.Errorf("Found zero address for node %s to get IP for service %s/%s of type %s", node.Name, namespace, serviceName, svc.Spec.Type)
		}
		return "https://" + node.Status.Addresses[0].Address + ":" + fmt.Sprint(svc.Spec.Ports[0].NodePort), nil
	}
	return "", fmt.Errorf("Unable to get IP for service %s/%s of unsupported type %s", namespace, serviceName, svc.Spec.Type)
}

func httpGet(requestURI string) ([]byte, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	}

	resp, err := client.Get(requestURI)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Error while requesting GET %s", requestURI))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Invalid HTTP Status received for GET %s: %d", requestURI, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed reading response")
	}

	return body, nil
}

// GetResourceCoverage is a helper method to get Coverage data for a resource from the service webhook.
func GetResourceCoverage(webhookURI string, gvk schema.GroupVersionKind) (string, error) {
	// TODO(spiffxp): this needs to handle same Kind, different GroupVersion (eg: CreationOptions, ListOptions, knative Service vs. k8s Service)
	requestURI := fmt.Sprintf(WebhookResourceCoverageEndPoint, webhookURI, gvk.Kind)
	body, err := httpGet(requestURI)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// GetAndWriteResourceCoverage is a helper method that uses GetResourceCoverage to get coverage and write it to a file.
func GetAndWriteResourceCoverage(webhookURI string, gvk schema.GroupVersionKind, outputFile string) error {
	resourceCoverage, err := GetResourceCoverage(webhookURI, gvk)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(outputFile, []byte(resourceCoverage), 0400)
}

// GetTotalCoverage calls the total coverage API to retrieve total coverage values.
func GetTotalCoverage(webhookURI string) (coveragecalculator.CoverageValues, error) {
	coverage := coveragecalculator.CoverageValues{}

	requestURI := fmt.Sprintf(WebhookTotalCoverageEndPoint, webhookURI)
	body, err := httpGet(requestURI)
	if err != nil {
		return coverage, err
	}

	if err = json.Unmarshal(body, &coverage); err != nil {
		return coverage, errors.Wrap(err, "Failed unmarshalling response to CoverageValues instance")
	}
	return coverage, nil
}

// GetAndWriteTotalCoverage uses the GetTotalCoverage method to get total coverage and write it to a output file.
func GetAndWriteTotalCoverage(webhookURI string, outputFile string) error {
	totalCoverage, err := GetTotalCoverage(webhookURI)
	if err != nil {
		return err
	}

	htmlData, err := view.GetHTMLCoverageValuesDisplay(totalCoverage)
	if err != nil {
		return errors.Wrap(err, "Failed building html file from total coverage. error")
	}

	return ioutil.WriteFile(outputFile, []byte(htmlData), 0400)
}

// GetResourcePercentages calls resource percentage coverage API to retrieve
// percentage values.
func GetResourcePercentages(webhookURI string) (coveragecalculator.CoveragePercentages, error) {
	coveragePercentages := coveragecalculator.CoveragePercentages{}

	requestURI := fmt.Sprintf(WebhookResourcePercentageCoverageEndPoint, webhookURI)
	body, err := httpGet(requestURI)
	if err != nil {
		return coveragePercentages, err
	}

	if err = json.Unmarshal(body, &coveragePercentages); err != nil {
		return coveragePercentages, errors.Wrap(err, "Failed unmarshalling resource percentage coverage response")
	}
	return coveragePercentages, nil
}

// WriteResourcePercentages writes CoveragePercentages to JUnit XML output file
func WriteResourcePercentages(outputFile string,
	coveragePercentages coveragecalculator.CoveragePercentages) error {
	htmlData, err := view.GetCoveragePercentageXMLDisplay(coveragePercentages)
	if err != nil {
		errors.Wrap(err, "Failed building coverage percentage xml file")
	}

	return ioutil.WriteFile(outputFile, []byte(htmlData), 0400)
}

// CleanupJunitFiles cleans up any existing JUnit XML files, to ensure we only
// have one JUnit XML file providing the API Coverage summary
func CleanupJunitFiles(artifactsDir string) {
	filepath.Walk(artifactsDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && jUnitFileRegexExpr.MatchString(info.Name()) {
			os.Remove(path)
		}
		return nil
	})
}
