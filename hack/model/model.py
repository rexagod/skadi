from sklearn.ensemble import IsolationForest
from sklearn.datasets import make_blobs
from skl2onnx import convert_sklearn
from skl2onnx.common.data_types import FloatTensorType
import numpy as np

# -------------------------------------------------------------------
# CREATE MOCK DATA REPRESENTING POD RESOURCE USAGE
# -------------------------------------------------------------------
#
# We generate 300 samples clustered around (0.5, 0.5) to simulate
# "normal" CPU and memory usage patterns. Then, we add 10 outliers
# with higher values to represent misbehaving workloads.
X, _ = make_blobs(n_samples=300, centers=[[0.5, 0.5]], cluster_std=0.1)
X = np.vstack([X, np.random.uniform(low=1.5, high=2.0, size=(10, 2))])

# -------------------------------------------------------------------
# TRAIN AN ISOLATION FOREST ON THIS DATA
# -------------------------------------------------------------------
#
# Isolation Forest is an unsupervised model well-suited for anomaly
# detection. It works by randomly partitioning the feature space and
# identifying points that are more easily isolated.
# 
# Setting contamination=0.05 means we expect ~5% of the data to be
# anomalous. A fixed random_state ensures repeatable results.
model = IsolationForest(contamination=0.05, random_state=42)
model.fit(X)

# -------------------------------------------------------------------
# CONVERT THE MODEL TO ONNX FORMAT
# -------------------------------------------------------------------
#
# We define the input schema (a 2D float tensor with two features: CPU, memory)
# and convert the scikit-learn model to an ONNX graph.
#
# The target_opset maps required domains to version numbers:
# - '' refers to the core ONNX ops (like Add, Reshape), using version 13
# - 'ai.onnx.ml' refers to ML-specific operators used in sklearn models,
#   like those in Isolation Forest, pinned here to version 3 for compatibility.
initial_type = [('input', FloatTensorType([None, 2]))]
onnx_model = convert_sklearn(
    model,
    initial_types=initial_type,
    target_opset={'': 13, 'ai.onnx.ml': 3}
)

# -------------------------------------------------------------------
# SAVE THE ONNX MODEL TO A FILE
# -------------------------------------------------------------------
#
# This serialized file can be loaded by the sidecar using onnxruntime-go
# to perform real-time anomaly detection on pod resource metrics.
with open("model.onnx", "wb") as f:
    f.write(onnx_model.SerializeToString())
