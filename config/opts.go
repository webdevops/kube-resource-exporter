package config

import (
	"encoding/json"
	"time"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Debug       bool `long:"log.debug"    env:"LOG_DEBUG"  description:"debug mode"`
			Development bool `long:"log.devel"    env:"LOG_DEVEL"  description:"development mode"`
			Json        bool `long:"log.json"     env:"LOG_JSON"   description:"Switch log output to json format"`
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
