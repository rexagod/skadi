package anomaly

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metricsapi "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

var SchemeGroupVersion = schema.GroupVersion{
	Group:   "github.com/rexagod/skadi",
	Version: "v1alpha1",
}

// +k8s:deepcopy-gen=true

type ContainerMetricsWithAnomaly struct {
	metricsapi.ContainerMetrics
	AnomalyScore float32 `json:"anomaly_score"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PodMetricsWithAnomaly struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Timestamp         metav1.Time                   `json:"timestamp"`
	Window            metav1.Duration               `json:"window"`
	Containers        []ContainerMetricsWithAnomaly `json:"containers"`
}

func (pmwa *PodMetricsWithAnomaly) GetObjectKind() schema.ObjectKind {
	return &metav1.TypeMeta{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       "PodMetricsWithAnomaly",
	}
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PodMetricsWithAnomalyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodMetricsWithAnomaly `json:"items"`
}

func (pmwal *PodMetricsWithAnomalyList) GetObjectKind() schema.ObjectKind {
	return &metav1.TypeMeta{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       "PodMetricsWithAnomalyList",
	}
}
