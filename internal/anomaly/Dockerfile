# The ./anomaly Dockerfile is responsible for packaging the anomaly detection plugin.

FROM python:3.13-slim

WORKDIR /app

COPY requirements.txt ./

# s/sklearn/scikit-learn
RUN export SKLEARN_ALLOW_DEPRECATED_SKLEARN_PACKAGE_INSTALL=True && \
    python -m venv venv && \
    ./venv/bin/pip install --upgrade pip && \
    ./venv/bin/pip install -r requirements.txt

COPY anomaly.py ./

ENV PATH="/app/venv/bin:$PATH"

EXPOSE 5001

CMD ["gunicorn", "-w", "4", "-b", "127.0.0.1:5001", "anomaly:app"]
