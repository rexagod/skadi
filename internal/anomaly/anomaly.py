from flask import Flask, request, jsonify
from qdrant_client import QdrantClient
from qdrant_client.http.models import PointStruct, Distance, VectorParams
import numpy as np
import os

app = Flask(__name__)

COLLECTION_NAME = "metrics"
VECTOR_SIZE = 2

QDRANT_HOST = os.environ.get("QDRANT_HOST", "127.0.0.1")
QDRANT_PORT = int(os.environ.get("QDRANT_PORT", 6333))
client = QdrantClient(host=QDRANT_HOST, port=QDRANT_PORT)

def ensure_collection():
    if not client.collection_exists(collection_name=COLLECTION_NAME):
        client.create_collection(
            collection_name=COLLECTION_NAME,
            vectors_config=VectorParams(size=VECTOR_SIZE, distance=Distance.EUCLID)
        )

def normalize(cpu, memory):
    CPU_MIN = float(os.environ.get("QDRANT_CPU_MIN", 0.0))
    CPU_MAX = float(os.environ.get("QDRANT_CPU_MAX", 30000000.0))  # 30,000,000n
    MEM_MIN = float(os.environ.get("QDRANT_MEM_MIN", 0.0))
    MEM_MAX = float(os.environ.get("QDRANT_MEM_MAX", 300000.0))  # 300,000Ki
    norm_cpu = (cpu - CPU_MIN) / (CPU_MAX - CPU_MIN) if CPU_MAX > CPU_MIN else 0.0
    norm_mem = (memory - MEM_MIN) / (MEM_MAX - MEM_MIN) if MEM_MAX > MEM_MIN else 0.0
    return [norm_cpu, norm_mem]

def parse_and_normalize(cpu, memory):
    # Assume cpu is always in n (nanocores) and memory is always in Ki
    cpu_val = float(cpu)
    mem_val = float(memory)
    return normalize(cpu_val, mem_val)

@app.route('/snapshot', methods=['POST'])
def snapshot():
    data = request.get_json()
    cpu = data.get('cpu')
    memory = data.get('memory')
    if cpu is None or memory is None:
        return jsonify({'error': 'Missing cpu or memory'}), 400
    vector = parse_and_normalize(cpu, memory)
    ensure_collection()
    try:
        point = PointStruct(id=np.random.randint(1, 1e12), vector=vector, payload={})
        client.upsert(collection_name=COLLECTION_NAME, points=[point])
        return jsonify({'status': 'ok'})
    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/predict', methods=['POST'])
def predict():
    data = request.get_json()
    cpu = data.get('cpu')
    memory = data.get('memory')
    if cpu is None or memory is None:
        return jsonify({'error': 'Missing cpu or memory'}), 400
    vector = parse_and_normalize(cpu, memory)
    ensure_collection()
    try:
        search_result = client.search(
            collection_name=COLLECTION_NAME,
            query_vector=vector,
            limit=5
        )
        if not search_result:
            return jsonify({'anomaly_score': 0.0})
        avg_dist = np.mean([point.score for point in search_result])
        return jsonify({'anomaly_score': float(avg_dist)})
    except Exception as e:
        return jsonify({'error': str(e)}), 500

if __name__ == '__main__':
    # For production, use: gunicorn -w 4 -b 127.0.0.1:5001 anomaly:app
    app.run(host=QDRANT_HOST, port=5001)
