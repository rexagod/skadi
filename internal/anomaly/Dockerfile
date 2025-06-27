# The ./anomaly Dockerfile is responsible for packaging the anomaly detection plugin.

FROM python:3.13-slim

WORKDIR /app

# Copy requirements and install dependencies
COPY requirements.txt ./
RUN python -m venv venv && \
    ./venv/bin/pip install --upgrade pip && \
    ./venv/bin/pip install -r requirements.txt

# Copy the anomaly.py script
COPY anomaly.py ./

# Set environment variables for venv
ENV PATH="/app/venv/bin:$PATH"

EXPOSE 5001

CMD ["gunicorn", "-w", "4", "-b", "127.0.0.1:5001", "anomaly:app"]
