GO ?= $(shell which go)
OS ?= $(shell $(GO) env GOOS)
ARCH ?= $(shell $(GO) env GOARCH)

IMAGE_NAME := "anx-cr.io/se-public/cert-manager-webhook-anexia"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/_out

KUBE_VERSION=1.25.0

$(shell mkdir -p "$(OUT)")
export TEST_ASSET_ETCD=_test/kubebuilder/etcd
export TEST_ASSET_KUBE_APISERVER=_test/kubebuilder/kube-apiserver
export TEST_ASSET_KUBECTL=_test/kubebuilder/kubectl

test: _test/kubebuilder
	$(GO) test -v -tags=integration,unit . -coverprofile coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

test-unit:
	$(GO) test -v . -coverprofile coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

test-integration: _test/kubebuilder
	$(GO) test -v -tags=integration . -coverprofile coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

_test/kubebuilder:
	curl -fsSL https://go.kubebuilder.io/test-tools/$(KUBE_VERSION)/$(OS)/$(ARCH) -o kubebuilder-tools.tar.gz
	mkdir -p _test/kubebuilder
	tar -xvf kubebuilder-tools.tar.gz
	mv kubebuilder/bin/* _test/kubebuilder/
	rm kubebuilder-tools.tar.gz
	rm -R kubebuilder

clean: clean-kubebuilder

clean-kubebuilder:
	rm -Rf _test/kubebuilder

build:
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml:
	helm template \
	    --name cert-manager-webhook-anexia \
      			--set image.repository=$(IMAGE_NAME) \
        		--set image.tag=$(IMAGE_TAG) \
        		deploy/cert-manager-webhook-anexia > "$(OUT)/rendered-manifest.yaml"
