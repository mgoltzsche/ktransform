PKG=github.com/mgoltzsche/ktransform
TEST_IMAGE=ktransform-test
TEST_NAMESPACE=ktransform-test-$(shell date '+%Y%m%d-%H%M%S')
define TESTDOCKERFILE
	FROM $(TEST_IMAGE)
	ENV K8S_VERSION=v1.18.5
	RUN apk add --update --no-cache git \
		&& curl -fsSLo /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/$${K8S_VERSION}/bin/linux/amd64/kubectl \
		&& chmod +x /usr/local/bin/kubectl
endef
export TESTDOCKERFILE


all: operator

operator:
	docker build --force-rm -t image-registry-operator -f build/Dockerfile --target=operator .

containerized-unit-tests:
	docker build --force-rm -f build/Dockerfile .

unit-tests:
	go test -v ./pkg/...

containerized-%: test-image
	$(eval DOCKER ?= $(if $(shell docker -v),docker,podman))
	$(eval DOPTS := $(if $(wildcard $(HOME)/.minikube),-v $(HOME)/.minikube:$(HOME)/.minikube,))
	$(DOCKER) run --rm --net host \
		-v "`pwd`:/go/src/$(PKG)" \
		--mount "type=bind,src=$$KUBECONFIG,dst=/root/.kube/config" \
		$(DOPTS) \
		--entrypoint /bin/sh $(TEST_IMAGE) -c "make $*"

test-image:
	docker build --force-rm -t $(TEST_IMAGE) -f build/Dockerfile --target=builddeps .
	echo "$$TESTDOCKERFILE" | docker build --force-rm -t $(TEST_IMAGE) -f - .

generate:
	#operator-sdk add api --api-version=ktransform.mgoltzsche.github.com/v1alpha1 --kind=SecretTransform
	#operator-sdk add controller --api-version=ktransform.mgoltzsche.github.com/v1alpha1 --kind=SecretTransform
	operator-sdk generate k8s
	operator-sdk generate crds

e2e-tests: operatorsdk-tests-local

operatorsdk-tests-local:
	kubectl create namespace $(TEST_NAMESPACE)-local
	operator-sdk test local ./test/e2e --operator-namespace $(TEST_NAMESPACE)-local --up-local; \
	STATUS=$$?; \
	kubectl delete namespace $(TEST_NAMESPACE)-local; \
	exit $$STATUS

operatorsdk-tests:
	kubectl create namespace $(TEST_NAMESPACE)
	operator-sdk test local ./test/e2e; \
	STATUS=$$?; \
	kubectl delete namespace $(TEST_NAMESPACE); \
	exit $$STATUS

install-tools: download-deps
	cat tools.go | grep -E '^\s*_' | cut -d'"' -f2 | xargs -n1 go install

download-deps:
	go mod download

clean:
	rm -rf build/_output .kubeconfig

start-minikube:
	minikube start --kubernetes-version=1.18.5 --network-plugin=cni --enable-default-cni --container-runtime=cri-o --bootstrapper=kubeadm

delete-minikube:
	minikube delete
