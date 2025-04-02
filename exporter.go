package main

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"go.uber.org/zap"
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

	baseLabels := []string{"gvk", "name", "namespace"}

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

	for _, metricConfig := range exporterConfig.Metrics {
		metricName := metricConfig.Metric.Name

		contextLogger := logger.With(
			zap.String("metric", metricName),
		)

		metric := m.Collector.GetMetricList(metricName)

		resources, err := k8sDyanmicClient.Resource(metricConfig.Resource).List(m.Context(), metricConfig.KubeMetaListOptions())
		if err != nil {
			contextLogger.Error(err)
			continue
		}

		for _, resource := range resources.Items {
			resourceLogger := contextLogger.With(
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

			metricLabels := map[string]string{
				"gvk": fmt.Sprintf(
					"%s/%s/%s",
					resource.GetObjectKind().GroupVersionKind().Group,
					resource.GetObjectKind().GroupVersionKind().Version,
					resource.GetObjectKind().GroupVersionKind().Kind,
				),
				"name":      resource.GetName(),
				"namespace": resource.GetNamespace(),
			}

			// find value
			if metricConfig.Metric.Value.MetricPathConfig != nil && metricConfig.Metric.Value.Path != "" {
				valuePath, err := metricConfig.Metric.Value.JsonPath()
				if err != nil {
					resourceLogger.Error(err)
				}

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

				if labelConfig.MetricPathConfig != nil && labelConfig.Path != "" {
					labelPath, err := labelConfig.JsonPath()

					if err != nil {
						resourceLogger.Error(err)
					}

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
	}
}
