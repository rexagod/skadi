IMAGE_TAG ?= $(shell git rev-parse --short=6 HEAD)

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

PYTHON ?= python3

CONTROLLER_GEN = $(shell which controller-gen)
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

.PHONY: run-anomaly
run-anomaly:
	@# Run the model using `gunicorn` in production mode: `gunicorn -w 4 -b 127.0.0.1:5001 anomaly:app`
	$(VENV_PYTHON) $(MODEL_GEN)

.PHONY: controller-gen
controller-gen:
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
