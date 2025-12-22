package main

import (
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/remeh/sizedwaitgroup"
	"github.com/webdevops/go-common/prometheus/collector"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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

	// generate metric gauges
	for _, resourceConfig := range exporterConfig.Resources {
		for _, metricConfig := range resourceConfig.Metrics {
			metricName := metricConfig.Name
			metricLabels := []string{}
			for labelName := range metricConfig.Labels {
				metricLabels = append(metricLabels, labelName)
			}

			gaugeVec := prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: metricName,
					Help: metricConfig.Help,
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
}

func (m *MetricsCollectorKubeResources) Reset() {}

func (m *MetricsCollectorKubeResources) Collect(callback chan<- func()) {
	wg := sizedwaitgroup.New(Opts.Metrics.ListParallelism)

	for _, resourceConfig := range exporterConfig.Resources {
		wg.Add()
		go func() {
			defer wg.Done()
			contextLogger := m.Logger().With(
				slog.String("gvr", fmt.Sprintf("%s/%s/%s", resourceConfig.Group, resourceConfig.Version, resourceConfig.Resource)),
			)

			m.collectResource(resourceConfig, contextLogger, callback)
		}()
	}

	wg.Wait()
}

func (m *MetricsCollectorKubeResources) collectResource(resourceConfig *config.ConfigResource, logger *slog.Logger, callback chan<- func()) {
	listOpts := resourceConfig.KubeMetaListOptions()

	if Opts.Metrics.ListLimit != nil {
		listOpts.Limit = *Opts.Metrics.ListLimit
	}

	for {
		result, err := k8sDyanmicClient.Resource(*resourceConfig.GroupVersionResource).List(m.Context(), listOpts)
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		listOpts.Continue = result.GetContinue()

		for _, resource := range result.Items {
			for _, metricConfig := range resourceConfig.Metrics {
				metricLogger := logger.With(
					slog.String("resource", fmt.Sprintf("%s/%s", resource.GetNamespace(), resource.GetName())),
					slog.String("metric", metricConfig.Name),
				)

				m.collectResourceMetric(resourceConfig, metricConfig, resource, metricLogger, callback)
			}
		}

		// check if we have more elements
		if listOpts.Continue == "" {
			break
		}
	}
}

func (m *MetricsCollectorKubeResources) collectResourceMetric(resourceConfig *config.ConfigResource, metricConfig *config.ConfigMetric, resource unstructured.Unstructured, logger *slog.Logger, callback chan<- func()) {
	metric := m.Collector.GetMetricList(metricConfig.Name)

	if !metricConfig.IsValidObject(resource) {
		logger.Debug("filtered")
		return
	}

	var metricValue *float64
	if metricConfig.Value.Value != nil {
		metricValue = metricConfig.Value.Value
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
	if valuePath := metricConfig.Value.JsonPath(); valuePath != nil {
		if results, err := valuePath.FindResults(resource.Object); err == nil {
			if len(results) == 1 && len(results[0]) == 1 {
				val := results[0][0].Interface()

				if v := metricConfig.Value.ParseValue(val); v != nil {
					metricValue = v
				}
			}
		} else {
			logger.Error(err.Error())
			return
		}
	}

	// find labels
	for labelName, labelConfig := range metricConfig.Labels {
		metricLabels[labelName] = labelConfig.Value

		if labelPath := labelConfig.JsonPath(); labelPath != nil {
			if results, err := labelPath.FindResults(resource.Object); err == nil {
				if len(results) == 1 && len(results[0]) == 1 {
					val := results[0][0].Interface()

					metricLabels[labelName] = labelConfig.ParseLabel(val)
				}
			} else {
				logger.Error(err.Error())
				return
			}
		}
	}

	// process metric
	if metricValue != nil {
		metric.Add(metricLabels, *metricValue)
	} else {
		logger.Debug("no value found")
	}
}
