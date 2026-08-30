package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/hostwithquantum/k8s-storage-exporter/internal/exporter"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cmd := &cli.Command{
		Name:  "k8s-storage-exporter",
		Usage: "export pod ephemeral storage and volume stats from the kubelet stats/summary API",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "listen-address",
				Value: ":9110",
				Usage: "address to serve metrics on",
			},
			&cli.StringFlag{
				Name:    "kubeconfig",
				Sources: cli.EnvVars("KUBECONFIG"),
				Usage:   "path to a kubeconfig; empty uses in-cluster config",
			},
			&cli.StringFlag{
				Name:    "node",
				Sources: cli.EnvVars("NODE_NAME"),
				Usage:   "scrape only this node (DaemonSet mode); empty scrapes all nodes",
			},
			&cli.DurationFlag{
				Name:  "scrape-timeout",
				Value: 30 * time.Second,
				Usage: "timeout for collecting stats",
			},
			&cli.BoolFlag{
				Name:  "disable-exporter-metrics",
				Usage: "exclude metrics about the exporter itself (go_*, process_*)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return run(ctx, cmd, log)
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := cmd.Run(ctx, os.Args); err != nil {
		log.Error("exporter failed", "error", err)
		os.Exit(1)
	}

	log.Info("Bye.")
}

// buildConfig uses an explicit kubeconfig (--kubeconfig or $KUBECONFIG) if
// given, otherwise in-cluster config. It never reads ~/.kube/config.
func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("not in a cluster and no --kubeconfig/$KUBECONFIG set: %w", err)
	}
	return cfg, nil
}

func run(ctx context.Context, cmd *cli.Command, log *slog.Logger) error {
	cfg, err := buildConfig(cmd.String("kubeconfig"))
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	if node := cmd.String("node"); node != "" {
		log = log.With("node", node)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(exporter.New(client, cmd.String("node"), cmd.Duration("scrape-timeout"), log))
	if !cmd.Bool("disable-exporter-metrics") {
		registry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:              cmd.String("listen-address"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Info("listening", "address", server.Addr, "apiserver", cfg.Host)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
