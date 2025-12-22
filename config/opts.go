package config

import (
	"encoding/json"
	"time"
)

type (
	Opts struct {
		Version struct {
			Version  bool    `long:"version" description:"Show version"`
			Template *string `long:"version.template" description:"Version go template, eg {{.Version}}"`
		}

		// logger
		Logger struct {
			Level  string `long:"log.level"    env:"LOG_LEVEL"   description:"Log level" choice:"trace" choice:"debug" choice:"info" choice:"warning" choice:"error" default:"info"`                          // nolint:staticcheck // multiple choices are ok
			Format string `long:"log.format"   env:"LOG_FORMAT"  description:"Log format" choice:"logfmt" choice:"json" default:"logfmt"`                                                                     // nolint:staticcheck // multiple choices are ok
			Source string `long:"log.source"   env:"LOG_SOURCE"  description:"Show source for every log message (useful for debugging and bug reports)" choice:"" choice:"short" choice:"file" choice:"full"` // nolint:staticcheck // multiple choices are ok
			Color  string `long:"log.color"    env:"LOG_COLOR"   description:"Enable color for logs" choice:"" choice:"auto" choice:"yes" choice:"no"`                                                        // nolint:staticcheck // multiple choices are ok
			Time   bool   `long:"log.time"     env:"LOG_TIME"    description:"Show log time"`
		}

		// kubernetes settings
		Kubernetes struct {
			Config string `long:"kubeconfig"            env:"KUBECONFIG"               description:"Kuberentes config path (should be empty if in-cluster)"`
		}

		Metrics struct {
			Labels struct {
				Name      string `long:"metric.label.name"          env:"METRIC_LABEL_NAME"      description:"Label for resource name"                 default:"name"`
				Namespace string `long:"metric.label.namespace"     env:"METRIC_LABEL_NAMESPACE" description:"Label for resource namespace"            default:"namespace"`
				Gvr       string `long:"metric.label.gvr"           env:"METRIC_LABEL_GVR"       description:"Label for resource GroupVersionResource" default:"gvr"`
			}

			ListLimit       *int64 `long:"metric.list.limit"  env:"METRIC_LIST_LIMIT"    description:"Result limit for list calls to reduce server stress (paging)"`
			ListParallelism int    `long:"metric.parallelism"  env:"METRIC_PARALLELISM"   description:"Defines how many metrics should be processed at the same time" default:"5"`
		}

		Scrape struct {
			Time time.Duration `long:"scrape.time"     env:"SCRAPE_TIME"    description:"Scrape time" default:"30m"`
		}

		Config struct {
			File string `long:"config"     env:"CONFIG"    description:"Path to config file" required:"true"`
		}

		// caching
		Cache struct {
			Path string `long:"cache.path" env:"CACHE_PATH" description:"Cache path (to folder, file://path... or azblob://storageaccount.blob.core.windows.net/containername or k8scm://{namespace}/{configmap}})"`
		}

		Server struct {
			// general options
			Bind         string        `long:"server.bind"              env:"SERVER_BIND"           description:"Server address"        default:":8080"`
			ReadTimeout  time.Duration `long:"server.timeout.read"      env:"SERVER_TIMEOUT_READ"   description:"Server read timeout"   default:"5s"`
			WriteTimeout time.Duration `long:"server.timeout.write"     env:"SERVER_TIMEOUT_WRITE"  description:"Server write timeout"  default:"10s"`
		}
	}
)

func (o *Opts) GetCachePath(path string) (ret *string) {
	if o.Cache.Path != "" {
		tmp := o.Cache.Path + "/" + path
		ret = &tmp
	}

	return
}

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}
