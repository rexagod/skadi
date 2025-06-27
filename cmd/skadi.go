package main

import (
	"context"
	"flag"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/didip/tollbooth/v8"
	"github.com/rexagod/skadi/internal/anomaly"
	"k8s.io/klog/v2"
	metricsapi "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/didip/tollbooth/v8/limiter"
)

func main() {
	ctx := klog.NewContext(notifier(), klog.NewKlogr())
	logger := klog.FromContext(ctx).WithName("skadi")

	flagSet := flag.NewFlagSet("skadi", flag.ExitOnError)
	forwardingAddr := flagSet.String("forwarding-addr", "http://127.0.0.1:8001", "Address used to forward requests to metrics-server")
	listenPort := flagSet.String("listen-port", ":8002", "Address used to listen for incoming requests")
	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		klog.Fatalf("Error parsing flags: %v", err)
	}

	resourcePath := "/apis/" + metricsapi.SchemeGroupVersion.String()
	podsPath := resourcePath + "/pods/" // anomaly score?
	nodesPath := resourcePath + "/nodes/"

	anomalyPlugin := anomaly.NewAnomalyPlugin(logger, *forwardingAddr, *listenPort, podsPath, nodesPath)
	anomalyPluginFlagSet := anomalyPlugin.FlagSet()
	err = anomalyPluginFlagSet.Parse(os.Args[1:])
	if err != nil {
		klog.Fatalf("Error parsing %s flags: %v", anomalyPlugin.Name(), err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc(podsPath+"anomalies", anomalyPlugin.HandlePodAnomalies(ctx))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		targetURL := *forwardingAddr + r.URL.Path
		req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
		if err != nil {
			http.Error(w, "failed to create fallback proxy request", http.StatusInternalServerError)
			return
		}
		req.Header = r.Header.Clone()

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "fallback proxy failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, v := range resp.Header {
			for _, val := range v {
				w.Header().Add(k, val)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	lmt := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Hour,
	}).SetIPLookup(limiter.IPLookup{
		Name:           "X-Real-IP",
		IndexFromRight: 0,
	})
	err = http.ListenAndServe(*listenPort, tollbooth.HTTPMiddleware(lmt)(mux))
	if err != nil {
		klog.Exitf("Failed to start server: %v", err)
	}
}

func notifier() context.Context {
	c := make(chan os.Signal, 2)
	ctx, cancel := context.WithCancel(context.Background())
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
		os.Exit(1)
	}()

	return ctx
}
