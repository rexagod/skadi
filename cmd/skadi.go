package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
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
	forwardingAddr := flagSet.String("forwarding-addr", "https://127.0.0.1:10250", "Address used to forward requests to metrics-server")
	listenPort := flagSet.String("listen-port", ":8002", "Address used to listen for incoming requests")
	tlsCert := flagSet.String("tls-cert", "/tmp/tls/tls.crt", "Path to proxy's TLS certificate")
	tlsKey := flagSet.String("tls-key", "/tmp/tls/tls.key", "Path to proxy's TLS key")
	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		klog.Fatalf("Error parsing flags: %v", err)
	}

	resourcePath := "/apis/" + metricsapi.SchemeGroupVersion.String()
	podsPath := resourcePath + "/pods/"
	nodesPath := resourcePath + "/nodes/"

	tokenRaw, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		klog.Fatalf("failed to read token file: %v", err)
	}
	token := string(tokenRaw)

	anomalyPlugin := anomaly.NewAnomalyPlugin(logger, token, *forwardingAddr, *listenPort, podsPath, nodesPath)
	anomalyPluginFlagSet := anomalyPlugin.FlagSet()
	err = anomalyPluginFlagSet.Parse(os.Args[1:])
	if err != nil {
		klog.Fatalf("Error parsing %s flags: %v", anomalyPlugin.Name(), err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc(podsPath+"anomalies", anomalyPlugin.HandlePodAnomalies(ctx))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		klog.Infof("Proxying request: %s %s", r.Method, r.URL.Path)

		req, err := http.NewRequestWithContext(r.Context(), r.Method, *forwardingAddr+r.URL.Path, r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create request: %v", err), http.StatusInternalServerError)
			return
		}
		req.Header = r.Header.Clone()
		req.Header.Set("Authorization", "Bearer "+token)

		insecureClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		resp, err := insecureClient.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to forward request: %v", err), http.StatusBadGateway)
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
	err = http.ListenAndServeTLS(*listenPort, *tlsCert, *tlsKey, tollbooth.HTTPMiddleware(lmt)(mux))
	if err != nil {
		klog.Exitf("Failed to start HTTPS server: %v", err)
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
