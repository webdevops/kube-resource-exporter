package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"

	yaml "github.com/goccy/go-yaml"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/zapr"
	flags "github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/webdevops/go-common/prometheus/collector"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/webdevops/kube-resource-exporter/config"
)

const (
	Author    = "webdevops.io"
	UserAgent = "kube-resource-exporter/"
)

var (
	argparser *flags.Parser
	Opts      config.Opts

	k8sDyanmicClient dynamic.Interface

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"

	// cache config
	cacheTag = "v2"

	exporterConfig *config.Config
)

func main() {
	initArgparser()
	defer initLogger().Sync() // nolint:errcheck

	logger.Infof("starting kube-resource-exporter v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), Author)
	logger.Info(string(Opts.GetJson()))

	initSystem()
	initConfig(Opts.Config.File)

	logger.Infof("init Kubernetes connection")
	initKubeConnection()

	logger.Infof("starting metrics collection")
	initMetricCollector()

	logger.Infof("starting http server on %s", Opts.Server.Bind)
	startHttpServer()
}

func initArgparser() {
	argparser = flags.NewParser(&Opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		var flagsErr *flags.Error
		if ok := errors.As(err, &flagsErr); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}
}

func initConfig(path string) {
	exporterConfig = &config.Config{}

	logger.With(zap.String("path", path)).Infof("reading configuration from file %v", path)
	/* #nosec */
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Fatal(err)
	}

	logger.With(zap.String("path", path)).Info("parsing configuration")
	err = yaml.UnmarshalWithOptions(data, exporterConfig, yaml.Strict(), yaml.UseJSONUnmarshaler())
	if err != nil {
		logger.Fatal(err)
	}

	if err := exporterConfig.Compile(); err != nil {
		logger.Fatal(err)
	}
}

func initKubeConnection() {
	var err error
	var config *rest.Config

	if kubeconfig := Opts.Kubernetes.Config; kubeconfig != "" {
		// KUBECONFIG
		config, err = clientcmd.BuildConfigFromFlags("", Opts.Kubernetes.Config)
		if err != nil {
			panic(err.Error())
		}
	} else {
		// K8S in cluster
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	// create kubernetes client
	k8sDyanmicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	log.SetLogger(zapr.NewLogger(logger.Desugar()))
}

func initMetricCollector() {
	collectorName := "kube-resources"
	c := collector.New(collectorName, &MetricsCollectorKubeResources{}, logger)
	c.SetScapeTime(Opts.Scrape.Time)
	c.SetCache(
		Opts.GetCachePath(collectorName+".json"),
		collector.BuildCacheTag(cacheTag, Opts.Metrics, exporterConfig),
	)
	if err := c.Start(); err != nil {
		logger.Fatal(err.Error())
	}
}

// start and handle prometheus handler
func startHttpServer() {
	mux := http.NewServeMux()

	// healthz
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err)
		}
	})

	// readyz
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err)
		}
	})

	mux.Handle("/metrics", collector.HttpWaitForRlock(promhttp.Handler()))

	srv := &http.Server{
		Addr:         Opts.Server.Bind,
		Handler:      mux,
		ReadTimeout:  Opts.Server.ReadTimeout,
		WriteTimeout: Opts.Server.WriteTimeout,
	}
	logger.Fatal(srv.ListenAndServe())
}
