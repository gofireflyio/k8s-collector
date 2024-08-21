package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/thoas/go-funk"
	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gofireflyio/k8s-collector/collector"
	"github.com/gofireflyio/k8s-collector/collector/config"
	"github.com/gofireflyio/k8s-collector/collector/helm"
	"github.com/gofireflyio/k8s-collector/collector/k8s"
	"github.com/gofireflyio/k8s-collector/collector/k8stypes"
)

const (
	defaultNamespace           = "firefly"
	defaultFireflyPodPrefix    = "firefly-cronjob"
	defaultInfralightPodPrefix = "infralight-cronjob"
)

func main() {
	// Parse command line flags
	ctx := context.Background()
	debug := flag.Bool("debug", false, "sets log level to debug")
	external := flag.String(
		"external",
		"",
		"run outside of the cluster (provide path to kubeconfig file)",
	)
	configDir := flag.String("config", "/etc/config", "configuration files directory")
	dryRun := flag.Bool("dry-run", false, "dry run (do not send anything to Firefly)")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Printf(
			"Firefly k8s-collector v%s, compiled %s\n",
			collector.Version,
			collector.BuildDate,
		)
		os.Exit(0)
	}

	// Initiate a logger
	logger := loadLogger(*debug)

	// Get cluster ID from command line arguments or environment variable.
	// The cluster ID is required.
	clusterID := flag.Arg(0)
	if clusterID == "" {
		clusterID = os.Getenv("CLUSTER_ID")
	}
	if clusterID == "" {
		logger.Fatal().
			Msg("Cluster ID must be provided either as a command line argument, or via the CLUSTER_ID environment variable")
	}

	// Load the collector configuration
	conf, err := config.LoadConfig(logger, nil, *configDir, *dryRun)
	if err != nil {
		logger.Panic().
			Err(err).
			Msg("Failed loading collector configuration")
	}

	apiConfig, err := loadKubeConfig(*external)
	if err != nil {
		logger.Panic().
			Err(err).
			Msg("Failed loading Kubernetes configuration")
	}

	namespace := Namespace()

	// Load the Kubernetes collector
	k8sCollector, err := k8s.DefaultConfiguration(apiConfig)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("Failed loading Kubernetes collector")
	}

	k8sTypesCollector, err := k8stypes.DefaultConfiguration(apiConfig)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("Failed loading Kubernetes collector")
	}

	// Load the Helm collector
	helmCollector, err := helm.DefaultConfiguration(logger.Printf)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("Failed loading Helm collector")
	}

	kClient, err := kubernetes.NewForConfig(apiConfig)
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("Failed to create kubernetes client")
	}

	if _, err = kClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); err == nil {
		jobs, err := kClient.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logger.Fatal().
				Err(err).
				Str("namespace", namespace).
				Msg("Failed to list kubernetes jobs in namespace")
		}
		running := funk.Filter(jobs.Items, func(job v1.Job) bool {
			return job.Status.Active == int32(1) && strings.Contains(job.Name, defaultNamespace) &&
				(strings.HasPrefix(job.Name, defaultFireflyPodPrefix) || strings.HasPrefix(job.Name, defaultInfralightPodPrefix))
		}).([]v1.Job)
		if len(running) > 1 {
			logger.Warn().
				Err(err).
				Str("namespace", namespace).
				Msg("Too many running collector jobs in namespace")
			return
		}
	} else {
		logger.Warn().
			Err(err).
			Str("namespace", namespace).
			Msg("Failed to get kubernetes namespace")
	}

	err = collector.
		New(clusterID, apiConfig, conf, k8sCollector, helmCollector, k8sTypesCollector).
		Run(context.TODO())
	if err != nil {
		logger.Fatal().
			Err(err).
			Msg("Fetcher failed")
	}

	logger.Info().Msg("Fetcher successfully finished")
}

func loadKubeConfig(external string) (apiConfig *rest.Config, err error) {
	// Load configuration for the Kubernetes API client. We are either running
	// from inside the cluster (i.e. inside a pod) or outside of the cluster.
	if external != "" {
		apiConfig, err = clientcmd.BuildConfigFromFlags("", external)
	} else {
		// Load configuration to connect to the Kubernetes API from within a K8s
		// cluster
		apiConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		return apiConfig, fmt.Errorf("failed loading Kubernetes configuration: %w", err)
	}
	return apiConfig, nil
}

func loadLogger(debug bool) *zerolog.Logger {
	// When running in debug mode, enable pretty-printed logging with minimum
	// log level set at DEBUG. In non-debug mode, use standard JSON logging with
	// unix timestamp for better performance
	if debug {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	}

	return &log.Logger
}

func Namespace() string {
	// This way assumes you've set the POD_NAMESPACE environment variable using the downward API.
	// This check has to be done first for backwards compatibility with the way InClusterConfig was originally set up
	if ns, ok := os.LookupEnv("POD_NAMESPACE"); ok {
		return ns
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return defaultNamespace
}
