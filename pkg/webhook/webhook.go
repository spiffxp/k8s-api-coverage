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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/markbates/inflect"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/webhook/certificates/resources"
	"sigs.k8s.io/k8s-api-coverage/pkg/kube"
)

var (
	// GroupVersionKind for deployment to be used to set the webhook's owner reference.
	deploymentKind = extensionsv1beta1.SchemeGroupVersion.WithKind("Deployment")
)

const (
	// webhook must be registered as a FQDN
	webhookDomain = "sigs.k8s.io"
	webhookPort   = 8443
)

// APICoverageWebhook encapsulates necessary configuration details for the api-coverage webhook.
type APICoverageWebhook struct {
	// WebhookName is the name of the validation webhook we create to intercept API calls.
	WebhookName string

	// ServiceName is the name of K8 service under which the webhook runs.
	ServiceName string

	// DeploymentName is the deployment name for the webhook.
	DeploymentName string

	// Namespace is the namespace in which everything above lives.
	Namespace string

	// Port where the webhook is served.
	Port int

	// RegistrationDelay controls how long validation requests
	// occurs after the webhook is started. This is used to avoid
	// potential races where registration completes and k8s apiserver
	// invokes the webhook before the HTTP server is started.
	RegistrationDelay time.Duration

	// ClientAuthType declares the policy the webhook server will follow for TLS Client Authentication.
	ClientAuth tls.ClientAuthType

	// CaCert is the CA Cert for the webhook server.
	CaCert []byte

	// FailurePolicy policy governs the webhook validation decisions.
	FailurePolicy admissionregistrationv1beta1.FailurePolicyType

	// Logger is the configured logger for the webhook.
	Logger *zap.SugaredLogger

	// KubeClient is the K8 client to the target cluster.
	KubeClient kubernetes.Interface
}

func (acw *APICoverageWebhook) generateServerConfig() (*tls.Config, error) {
	serverKey, serverCert, caCert, err := resources.CreateCerts(context.Background(), acw.ServiceName, acw.Namespace)
	if err != nil {
		return nil, fmt.Errorf("Error creating webhook certificates: %v", err)
	}

	cert, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, fmt.Errorf("Error creating X509 Key pair for webhook server: %v", err)
	}

	acw.CaCert = caCert
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   acw.ClientAuth,
	}, nil
}

func (acw *APICoverageWebhook) getWebhookServer(handler http.Handler) (*http.Server, error) {
	tlsConfig, err := acw.generateServerConfig()
	if err != nil {
		return nil, fmt.Errorf("Error generating server config: %v", err)
	}

	return &http.Server{
		Handler:   handler,
		Addr:      fmt.Sprintf(":%d", acw.Port),
		TLSConfig: tlsConfig,
	}, nil
}

func (acw *APICoverageWebhook) registerWebhook(rules []admissionregistrationv1beta1.RuleWithOperations, namespace string) error {
	acw.Logger.Info("APICoverageRecorder.registerWebhook")
	webhook := &admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      acw.WebhookName,
			Namespace: namespace,
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{{
			Name:  acw.WebhookName,
			Rules: rules,
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Namespace: namespace,
					Name:      acw.ServiceName,
				},
				CABundle: acw.CaCert,
			},
			FailurePolicy: &acw.FailurePolicy,
		},
		},
	}

	deployment, err := acw.KubeClient.AppsV1().Deployments(namespace).Get(acw.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Error retrieving Deployment Extension object: %v", err)
	}
	deploymentRef := metav1.NewControllerRef(deployment, deploymentKind)
	webhook.OwnerReferences = append(webhook.OwnerReferences, *deploymentRef)

	// TODO(spiffxp): seems like this should either delete a pre-existing
	// webhook, or ignore an existing webhook if it matches the same
	// deploymentRef we just tried to set
	_, err = acw.KubeClient.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().Create(webhook)
	if err != nil {
		return fmt.Errorf("Error creating ValidatingWebhookConfigurations object: %v", err)
	}

	return nil
}

func (acw *APICoverageWebhook) getValidationRules(gvks []schema.GroupVersionKind) []admissionregistrationv1beta1.RuleWithOperations {
	var rules []admissionregistrationv1beta1.RuleWithOperations
	for _, gvk := range gvks {
		resourcePlural := strings.ToLower(inflect.Pluralize(gvk.Kind))
		rules = append(rules, admissionregistrationv1beta1.RuleWithOperations{
			Operations: []admissionregistrationv1beta1.OperationType{
				admissionregistrationv1beta1.Create,
				admissionregistrationv1beta1.Update,
				admissionregistrationv1beta1.Connect,
				// We ignore DELETE because no body is sent along, so we have
				// nothing with which to compute field coverage
			},
			Rule: admissionregistrationv1beta1.Rule{
				APIGroups:   []string{gvk.Group},
				APIVersions: []string{gvk.Version},
				Resources:   []string{resourcePlural, resourcePlural + "/*"}, // we also want all subresources, if any exist
			},
		})
	}
	return rules
}

// Run sets up the webhook with the provided http.handler, resourcegroup Map, namespace and stop channel.
func (acw *APICoverageWebhook) Run(handler http.Handler, resources []schema.GroupVersionKind, namespace string, stop <-chan struct{}) error {
	acw.Logger.Info("APICoverageWebhook.Run")
	server, err := acw.getWebhookServer(handler)
	if err != nil {
		return fmt.Errorf("Webhook server object creation failed: %v", err)
	}

	select {
	case <-time.After(acw.RegistrationDelay):
		rules := acw.getValidationRules(resources)
		err = acw.registerWebhook(rules, namespace)
		if err != nil {
			return fmt.Errorf("Webhook registration failed: %v", err)
		}
		acw.Logger.Infof("Registered webhook %s/%s, owned by deployment %s/%s, pointing to service %s/%s",
			namespace, acw.WebhookName, namespace, acw.DeploymentName, namespace, acw.ServiceName)
	case <-stop:
		return nil
	}

	serverBootstrapErrCh := make(chan struct{})
	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil {
			acw.Logger.Error("ListenAndServeTLS for admission webhook returned error", zap.Error(err))
			close(serverBootstrapErrCh)
			return
		}
		acw.Logger.Infof("Started webhook server, listening on %s", server.Addr)
	}()

	select {
	case <-stop:
		return server.Close()
	case <-serverBootstrapErrCh:
		return errors.New("webhook server bootstrap failed")
	}
}

func buildLogger(name string, level string) (*zap.SugaredLogger, error) {
	loggingConfig := zap.NewProductionConfig()
	if level, err := levelFromString(level); err == nil {
		loggingConfig.Level = zap.NewAtomicLevelAt(*level)
	}
	logger, err := loggingConfig.Build()
	if err != nil {
		return nil, err
	}
	return logger.Sugar().Named(name), nil
}

func levelFromString(level string) (*zapcore.Level, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid logging level: %v", level)
	}
	return &zapLevel, nil
}

// BuildWebhookConfiguration builds the APICoverageWebhook object using the provided names.
func BuildWebhookConfiguration(componentCommonName string, namespace string) *APICoverageWebhook {
	logger, err := buildLogger("webhook", "info")
	if err != nil {
		log.Fatalf("Failed to build logger: %v", err)
	}
	logger.Info("Built logger")

	kubeClient, err := kube.BuildKubeClient()
	if err != nil {
		logger.Fatalf("Failed to get client set: %v", err)
	}
	logger.Info("Built kubernetes client")

	return &APICoverageWebhook{
		Logger:            logger,
		KubeClient:        kubeClient,
		FailurePolicy:     admissionregistrationv1beta1.Ignore, // TODO(spiffxp): this was Fail, I think it should be Ignore, or at least it needs to be while I debug
		ClientAuth:        tls.NoClientCert,
		RegistrationDelay: time.Second * 2,
		Port:              webhookPort,
		Namespace:         namespace,
		DeploymentName:    componentCommonName,
		ServiceName:       componentCommonName,
		WebhookName:       componentCommonName + "." + webhookDomain,
	}
}
