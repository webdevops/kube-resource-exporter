package main

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/remeh/sizedwaitgroup"
	"github.com/webdevops/go-common/prometheus/collector"
	"go.uber.org/zap"

	"github.com/webdevops/kube-resource-exporter/config"
)

type (
	MetricsCollectorKubeResources struct {
		collector.Processor

		prometheus struct {
			metric map[string]*prometheus.GaugeVec
		}
	}
)

func (m *MetricsCollectorKubeResources) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	baseLabels := []string{}

	if Opts.Metrics.Labels.Gvr != "" {
		baseLabels = append(baseLabels, Opts.Metrics.Labels.Gvr)
	}

	if Opts.Metrics.Labels.Namespace != "" {
		baseLabels = append(baseLabels, Opts.Metrics.Labels.Namespace)
	}

	if Opts.Metrics.Labels.Name != "" {
		baseLabels = append(baseLabels, Opts.Metrics.Labels.Name)
	}

	m.prometheus.metric = map[string]*prometheus.GaugeVec{}

	for _, row := range exporterConfig.Metrics {
		row.KubeMetaListOptions()
	}

	for _, metricConfig := range exporterConfig.Metrics {
		metricName := metricConfig.Metric.Name
		metricLabels := []string{}
		for labelName := range metricConfig.Metric.Labels {
			metricLabels = append(metricLabels, labelName)
		}

		gaugeVec := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricName,
				Help: metricConfig.Metric.Help,
			},
			append(
				baseLabels,
				metricLabels...,
			),
		)
		m.Collector.RegisterMetricList(metricName, gaugeVec, true)
		m.prometheus.metric[metricName] = gaugeVec
	}
}

func (m *MetricsCollectorKubeResources) Reset() {}

func (m *MetricsCollectorKubeResources) Collect(callback chan<- func()) {
	wg := sizedwaitgroup.New(Opts.Metrics.ListParallelism)

	for _, metricConfig := range exporterConfig.Metrics {
		wg.Add()
		go func() {
			defer wg.Done()
			contextLogger := logger.With(
				zap.String("metric", metricConfig.Metric.Name),
			)

			m.collectMetric(metricConfig, contextLogger, callback)
		}()
	}

	wg.Wait()
}

func (m *MetricsCollectorKubeResources) collectMetric(metricConfig *config.ConfigMetrics, logger *zap.SugaredLogger, callback chan<- func()) {
	metric := m.Collector.GetMetricList(metricConfig.Metric.Name)

	listOpts := metricConfig.KubeMetaListOptions()

	if Opts.Metrics.ListLimit != nil {
		listOpts.Limit = *Opts.Metrics.ListLimit
	}

	for {
		result, err := k8sDyanmicClient.Resource(metricConfig.Resource).List(m.Context(), listOpts)
		if err != nil {
			logger.Error(err)
			continue
		}
		listOpts.Continue = result.GetContinue()

		for _, resource := range result.Items {
			resourceLogger := logger.With(
				zap.String(
					"resource",
					fmt.Sprintf("%s/%s", resource.GetNamespace(), resource.GetName()),
				),
			)

			if !metricConfig.IsValidObject(resource) {
				resourceLogger.Debug("filtered")
				continue
			}

			var metricValue *float64
			if metricConfig.Metric.Value.Value != nil {
				metricValue = metricConfig.Metric.Value.Value
			}

			metricLabels := map[string]string{}

			if Opts.Metrics.Labels.Gvr != "" {
				metricLabels[Opts.Metrics.Labels.Gvr] = fmt.Sprintf(
					"%s/%s/%s",
					resource.GetObjectKind().GroupVersionKind().Group,
					resource.GetObjectKind().GroupVersionKind().Version,
					resource.GetObjectKind().GroupVersionKind().Kind,
				)
			}

			if Opts.Metrics.Labels.Namespace != "" {
				metricLabels[Opts.Metrics.Labels.Namespace] = resource.GetNamespace()
			}

			if Opts.Metrics.Labels.Name != "" {
				metricLabels[Opts.Metrics.Labels.Name] = resource.GetName()
			}

			// find value
			if valuePath := metricConfig.Metric.Value.JsonPath(); valuePath != nil {
				if results, err := valuePath.FindResults(resource.Object); err == nil {
					if len(results) == 1 && len(results[0]) == 1 {
						val := results[0][0].Interface()

						if v := metricConfig.Metric.Value.ParseValue(val); v != nil {
							metricValue = v
						}
					}
				} else {
					resourceLogger.Error(err)
				}
			}

			// find labels
			for labelName, labelConfig := range metricConfig.Metric.Labels {
				metricLabels[labelName] = labelConfig.Value

				if labelPath := labelConfig.JsonPath(); labelPath != nil {
					if results, err := labelPath.FindResults(resource.Object); err == nil {
						if len(results) == 1 && len(results[0]) == 1 {
							val := results[0][0].Interface()

							metricLabels[labelName] = labelConfig.ParseLabel(val)
						}
					} else {
						resourceLogger.Error(err)
					}
				}
			}

			// process metric
			if metricValue != nil {
				metric.Add(metricLabels, *metricValue)
			} else {
				resourceLogger.Debug("no value found")
			}
		}

		// check if we have more elements
		if listOpts.Continue == "" {
			break
		}
	}
}
