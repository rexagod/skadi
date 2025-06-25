MODEL_DIR=hack/model
MODEL_GEN=$(MODEL_DIR)/model.py
MODEL=model.onnx
MODEL_REQUIREMENTS=$(MODEL_DIR)/requirements.txt
VENV_DIR=hack/venv
VENV_PYTHON=$(VENV_DIR)/bin/python
VENV_PIP=$(VENV_DIR)/bin/pip

.PHONY: venv
$(VENV_PYTHON):
	python3 -m venv $(VENV_DIR) && \
	$(VENV_PIP) install -r $(MODEL_REQUIREMENTS)

$(MODEL): $(MODEL_GEN) $(MODEL_REQUIREMENTS) | $(VENV_PYTHON)
	$(VENV_PYTHON) $(MODEL_GEN)

.PHONY: clean-venv
clean-venv:
	rm -rf $(VENV_DIR) $(MODEL)

