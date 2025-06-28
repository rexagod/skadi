IMAGE_TAG ?= $(shell git rev-parse --short=7 HEAD)

MAIN = cmd/skadi

MAIN_DOCKERFILE=Dockerfile
MAIN_IMAGE=skadi:$(IMAGE_TAG)

MODEL_DIR=internal/anomaly
MODEL_GEN=$(MODEL_DIR)/anomaly.py
MODEL_CLIENT=$(MODEL_DIR)/anomaly.go
MODEL_REQUIREMENTS=$(MODEL_DIR)/requirements.txt

MODEL_PKG = pkg/anomaly
MODEL_DEEPCOPY_GEN = $(MODEL_PKG)/zz_generated.deepcopy.go
MODEL_TYPES = $(MODEL_PKG)/types.go

MODEL_DOCKERFILE=$(MODEL_DIR)/Dockerfile
MODEL_IMAGE=skadi-anomaly-plugin:$(IMAGE_TAG)

VENV_DIR=$(MODEL_DIR)/venv
VENV_PYTHON=$(VENV_DIR)/bin/python
VENV_PIP=$(VENV_DIR)/bin/pip

ASSETS_DIR=assets

TMP_DIR=/tmp
TLS_DIR=$(TMP_DIR)/tls

PYTHON ?= python3

CONTROLLER_GEN = $(GOPATH)/bin/controller-gen
CONTROLLER_GEN_VERSION ?= v0.18.0

$(MAIN): $(MAIN).go $(wildcard internal/**/*.go) go.mod go.sum
	go build -o $(MAIN) $(MAIN).go

$(VENV_DIR): $(MODEL_REQUIREMENTS)
	@rm -rf $(VENV_DIR) && \
	$(PYTHON) -m venv $(VENV_DIR) && \
	$(VENV_PIP) install -r $(MODEL_REQUIREMENTS)

$(MODEL_DEEPCOPY_GEN): $(MODEL_TYPES)
	$(CONTROLLER_GEN) object paths="./pkg/anomaly/..."

$(MAIN_DOCKERFILE): $(MAIN)
	docker build --load -t $(MAIN_IMAGE) . && \
	kind load docker-image $(MAIN_IMAGE) || true

$(MODEL_DOCKERFILE): $(MODEL_GEN) $(MODEL_CLIENT) $(MODEL_REQUIREMENTS)
	docker build --load -t $(MODEL_IMAGE) $(MODEL_DIR) && \
	kind load docker-image $(MODEL_IMAGE) || true

$(CONTROLLER_GEN):
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

.PHONY: tls token

tls:
	@# Generate TLS certificates for *local* development
	@mkdir -p $(TLS_DIR) && \
	openssl req -x509 -newkey rsa:4096 -keyout $(TLS_DIR)/tls.key -out $(TLS_DIR)/tls.crt -days 365 -nodes -subj "/CN=metrics-server.kube-system.svc"

token:
	@# Copy SA token to localhost
	METRICS_SERVER_POD=$(shell kubectl get pods -n kube-system -l k8s-app=metrics-server -o jsonpath='{.items[0].metadata.name}') && \
	kubectl exec -n kube-system metrics-server-5b75c7bbb4-mh96d -c skadi -- cat /var/run/secrets/kubernetes.io/serviceaccount/token > $$SA_TOKEN_PATH && \
	echo "Copied SA token to $$SA_TOKEN_PATH"
