GO ?= $(shell which go)
OS ?= $(shell $(GO) env GOOS)
ARCH ?= $(shell $(GO) env GOARCH)

IMAGE_NAME := "anx-cr.io/se-public/cert-manager-webhook-anexia"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/_out
LOCALBIN := $(shell pwd)/_test

KUBE_VERSION=1.30

$(shell mkdir -p "$(OUT)")
$(shell mkdir -p "$(LOCALBIN)")

ENVTEST ?= $(LOCALBIN)/setup-envtest

.PHONY: envtest
envtest: $(ENVTEST)
$(ENVTEST):
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

ENVTEST_KUBEBUILDER_ASSETS = KUBEBUILDER_ASSETS="$$($(ENVTEST) use $(KUBE_VERSION) --bin-dir $(LOCALBIN) -p path)"

test: envtest
	$(ENVTEST_KUBEBUILDER_ASSETS) $(GO) test -v -tags=integration,unit . -coverprofile coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

test-unit:
	$(GO) test -v . -coverprofile coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

test-integration: envtest
	$(ENVTEST_KUBEBUILDER_ASSETS) $(GO) test -v -tags=integration . -coverprofile coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

clean: clean-kubebuilder

clean-kubebuilder:
	rm -Rf _test

build:
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml:
	helm template \
	    --name-template cert-manager-webhook-anexia \
      			--set image.repository=$(IMAGE_NAME) \
        		--set image.tag=$(IMAGE_TAG) \
        		deploy/cert-manager-webhook-anexia > "$(OUT)/rendered-manifest.yaml"
