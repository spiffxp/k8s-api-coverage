# k8s-api-coverage

Is it possible to extract / decouplate knative's api coverage from knative?

The tooling / code is currently spread across
- knative/test-infra
- knative/pkg
- knative/serving

Goal:
- copy knative/serving/test/apicoverage/tools/main.go
- get it to build/run here with a go.mod
- copy in dependencies
- same
- same for whatever the webhook is
