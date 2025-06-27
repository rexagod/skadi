package anomaly

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/go-logr/logr"
	pkg "github.com/rexagod/skadi/pkg/anomaly"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

type (
	containerMetricsWithAnomaly = pkg.ContainerMetricsWithAnomaly
	podMetricsWithAnomaly       = pkg.PodMetricsWithAnomaly
	podMetricsWithAnomalyList   = pkg.PodMetricsWithAnomalyList
)

type Plugin struct {
	logger         logr.Logger
	forwardingAddr string
	listenAddr     string
	podsPath       string
	nodesPath      string
}

func NewAnomalyPlugin(logger logr.Logger, forwardingAddr, listenAddr, podsPath, nodesPath string) *Plugin {
	return &Plugin{
		logger:         logger,
		forwardingAddr: forwardingAddr,
		listenAddr:     listenAddr,
		podsPath:       podsPath,
		nodesPath:      nodesPath,
	}
}

func (p *Plugin) Name() string {
	return "anomaly"
}

var (
	enable              *bool
	pollInterval        *time.Duration
	thresholdPercentile *int
	modelAddress        *string
)

func (p *Plugin) FlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("anomaly-plugin", flag.ExitOnError)
	enable = fs.Bool(fmt.Sprintf("%s-enable", p.Name()), true, "Enable anomaly detection plugin")
	pollInterval = fs.Duration(fmt.Sprintf("%s-poll-interval", p.Name()), 10*time.Second, "Polling interval for snapshotting metrics (0 to disable polling)")
	thresholdPercentile = fs.Int(fmt.Sprintf("%s-threshold-percentile", p.Name()), 99, "Percentile threshold for anomaly detection (0 to disable thresholding)")
	modelAddress = fs.String(fmt.Sprintf("%s-model-address"), "http://127.0.0.1:5001", "Qdrant-based model's address for anomaly detection")

	return fs
}

type ScoreWrapper struct {
	AnomalyScore float32 `json:"anomaly_score"`
}

var syncOnce = &sync.Once{}

func (p *Plugin) HandlePodAnomalies(ctx context.Context) http.HandlerFunc {
	if !*enable {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Anomaly detection plugin is disabled", http.StatusNotImplemented)
		}
	}

	syncOnce.Do(func() {
		go p.repeatSnapshot(ctx)
	})

	return func(w http.ResponseWriter, r *http.Request) {
		err, anomalySet, scores := p.snapshotWithPrediction(true, true)

		var anomalies []podMetricsWithAnomaly
		if *thresholdPercentile <= 0 {
			threshold := calculateThreshold(scores, *thresholdPercentile)

			for _, anomaly := range anomalySet {
				var score float32
				for _, container := range anomaly.Containers {
					score += container.AnomalyScore
				}
				score /= float32(len(anomaly.Containers))
				if score >= threshold {
					anomalies = append(anomalies, anomaly)
				}
			}
		} else {
			anomalies = anomalySet
		}

		w.Header().Set("Content-Type", "application/json")

		anomaliesList := podMetricsWithAnomalyList{
			TypeMeta: v1.TypeMeta{
				Kind:       "PodMetricsWithAnomalyList",
				APIVersion: pkg.SchemeGroupVersion.String(),
			},
			ListMeta: v1.ListMeta{ /* empty */ },
			Items:    anomalies,
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", " ")
		err = encoder.Encode(anomaliesList)
		if err != nil {
			return
		}
	}
}

func calculateThreshold(data []float32, p int) float32 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]float32, len(data))
	copy(sorted, data)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	idx := (len(sorted) - 1) * p / 100
	return sorted[idx]
}

func (p *Plugin) repeatSnapshot(ctx context.Context) {
	if *pollInterval == 0 {
		p.logger.V(2).WithName("repeatSnapshot").Info("Polling for snapshot generation is disabled")
		return
	}
	for {
		select {
		case <-ctx.Done():
			p.logger.V(1).WithName("repeatSnapshot").Info("Stopping polling for snapshot generation")
			return
		default:
			err, _, _ := p.snapshotWithPrediction(true, false)
			if err != nil {
				p.logger.V(1).WithName("repeatSnapshot").Error(err, "Failed to generate snapshot")
			}
			time.Sleep(*pollInterval)
		}
	}
}

func (p *Plugin) snapshotWithPrediction(shouldSnapshot, shouldPredict bool) (err error, anomalies []podMetricsWithAnomaly, scores []float32) {
	if !shouldSnapshot && !shouldPredict {
		return
	}

	response, err := http.Get(p.forwardingAddr + p.podsPath)
	if err != nil {
		return fmt.Errorf("error fetching metrics from proxy URL: %w", err), nil, nil
	}
	defer response.Body.Close()

	var podMetrics metricsv1beta1.PodMetricsList
	err = json.NewDecoder(response.Body).Decode(&podMetrics)
	if err != nil {
		return fmt.Errorf("error decoding metrics from proxy URL: %w", err), nil, nil
	}
	for _, pod := range podMetrics.Items {
		containerScores := make([]float32, 0)
		containerMetrics := make([]containerMetricsWithAnomaly, 0)

		for _, container := range pod.Containers {
			var (
				gotCPUNanocores float64
				gotMemoryKibs   float64
			)
			_, err = fmt.Sscanf(container.Usage.Cpu().String(), "%fn", &gotCPUNanocores)
			if err != nil {
				return fmt.Errorf("error parsing CPU usage from proxy URL: %w", err), nil, nil
			}
			_, err = fmt.Sscanf(container.Usage.Memory().String(), "%fKi", &gotMemoryKibs)
			if err != nil {
				return fmt.Errorf("error parsing memory usage from proxy URL: %w", err), nil, nil
			}

			if shouldSnapshot {
				_, err = snapshotOrPredict(shouldSnapshot, false, gotCPUNanocores, gotMemoryKibs)
				if err != nil {
					return fmt.Errorf("error snapshotting metrics: %w", err), nil, nil
				}
			}

			if shouldPredict {
				result, err := snapshotOrPredict(false, shouldPredict, gotCPUNanocores, gotMemoryKibs)
				if err != nil {
					return fmt.Errorf("error predicting anomalies: %w", err), nil, nil
				}

				containerMetric := containerMetricsWithAnomaly{
					ContainerMetrics: container,
					AnomalyScore:     result.AnomalyScore,
				}
				containerMetrics = append(containerMetrics, containerMetric)

				containerScores = append(containerScores, result.AnomalyScore)
			}

			anomaly := podMetricsWithAnomaly{
				TypeMeta: v1.TypeMeta{
					Kind:       "PodMetricsWithAnomaly",
					APIVersion: pkg.SchemeGroupVersion.String(),
				},
				ObjectMeta: pod.ObjectMeta,
				Timestamp:  pod.Timestamp,
				Window:     pod.Window,
				Containers: containerMetrics,
			}
			anomalies = append(anomalies, anomaly)

			var podScore float32
			for _, score := range containerScores {
				podScore += score
			}
			scores = append(scores, podScore/float32(len(containerScores)))
		}
	}

	return
}

func snapshotOrPredict(shouldSnapshot, shouldPredict bool, gotCPUNanocores, gotMemoryKibs float64) (*ScoreWrapper, error) {
	var endpoint string

	if shouldSnapshot {
		endpoint = "/snapshot"
	} else if shouldPredict {
		endpoint = "/predict"
	} else {
		return nil, fmt.Errorf("no action specified for snapshot or predict")
	}

	payload := map[string]float64{"cpu": gotCPUNanocores, "memory": gotMemoryKibs}
	b, _ := json.Marshal(payload)
	response, err := http.Post(*modelAddress+endpoint, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("error making %s request: %w", endpoint, err)
	}
	defer response.Body.Close()

	result := &ScoreWrapper{}
	err = json.NewDecoder(response.Body).Decode(result)
	if err != nil {
		return nil, fmt.Errorf("error decoding %s response: %w", endpoint, err)
	}

	return result, nil
}
