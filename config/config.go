package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/jsonpath"
	"k8s.io/kubectl/pkg/cmd/get"
)

const (
	TYPE_TIMESTAMP = "timestamp"
	TYPE_DATETIME  = "datetime"

	KUBE_SELECTOR_ERROR = "<error>"
	KUBE_SELECTOR_NONE  = "<none>"
)

type (
	Config struct {
		Metrics []*ConfigMetrics `yaml:"metrics"`
	}

	ConfigMetrics struct {
		Metric   *MetricConfig               `yaml:"metric"`
		Resource schema.GroupVersionResource `yaml:"resource"`

		Selector  *metav1.LabelSelector `yaml:"selector"`
		_selector string

		Filters []*MetricFilterConfig `yaml:"filters"`
	}

	MetricConfig struct {
		Name   string                        `yaml:"name"`
		Help   string                        `yaml:"help"`
		Value  *MetricValueConfig            `yaml:"value"`
		Labels map[string]*MetricLabelConfig `yaml:"labels"`
	}

	MetricValueConfig struct {
		*MetricPathConfig `yaml:",inline"`
		Value             *float64 `yaml:"value"`
	}

	MetricLabelConfig struct {
		*MetricPathConfig `yaml:",inline"`
		Value             string `yaml:"value"`
	}

	MetricPathConfig struct {
		Path  string `yaml:"jsonPath" json:"jsonPath"`
		_path *jsonpath.JSONPath
		Type  *string `yaml:"type"`
	}

	MetricFilterConfig struct {
		Path  string `yaml:"jsonPath" json:"jsonPath"`
		_path *jsonpath.JSONPath
	}

	MetricJsonPath string
)

var (
	timeFormats = []string{
		// preferred format
		time.RFC3339,

		// human format
		"2006-01-02 15:04:05 +07:00",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05",

		// allowed formats
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339Nano,

		// least preferred format
		"2006-01-02",
	}
)

func (m *Config) Compile() error {
	for _, metric := range m.Metrics {
		err := metric.Compile()
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *ConfigMetrics) Compile() error {
	// value path
	if m.Metric.Value.MetricPathConfig != nil && m.Metric.Value.Path != "" {
		if path, err := compileJsonPath(m.Metric.Value.Path); err == nil {
			m.Metric.Value._path = path
		} else {
			return err
		}
	}

	// labels path
	for _, labelConfig := range m.Metric.Labels {
		if labelConfig.MetricPathConfig != nil && labelConfig.Path != "" {
			if path, err := compileJsonPath(labelConfig.Path); err == nil {
				labelConfig._path = path
			} else {
				return err
			}
		}
	}

	// filters
	for _, filterConfig := range m.Filters {
		if filterConfig.Path == "" {
			return fmt.Errorf(`jsonPath must be set for filters`)
		}

		if path, err := compileJsonPath(filterConfig.Path); err == nil {
			filterConfig._path = path
		} else {
			return err
		}
	}

	// selector
	if m.Selector != nil {
		selector := metav1.FormatLabelSelector(m.Selector)
		if strings.EqualFold(selector, KUBE_SELECTOR_ERROR) {
			return fmt.Errorf(`unable to compile Kubernetes selector for metric "%s"`, m.Metric.Name)
		}

		if !strings.EqualFold(selector, KUBE_SELECTOR_NONE) {
			m._selector = selector
		}
	}

	return nil
}

func (m *ConfigMetrics) KubeMetaListOptions() metav1.ListOptions {
	opts := metav1.ListOptions{}
	if m._selector != "" {
		opts.LabelSelector = m._selector
	}

	return opts
}

func (m *MetricPathConfig) JsonPath() (*jsonpath.JSONPath, error) {
	return m._path, nil
}

func (m *MetricPathConfig) ParseLabel(val interface{}) (ret string) {
	// convert type
	switch v := val.(type) {
	case float64:
		ret = fmt.Sprintf("%f", v)
	case int64:
		ret = fmt.Sprintf("%d", v)
	case string:
		ret = v
	}

	// parse/convert value
	if m.Type != nil {
		switch *m.Type {
		case TYPE_TIMESTAMP:
			// check if unixtimestamp
			if _, err := strconv.ParseFloat(ret, 64); err == nil {
				// already timestamp, keep it
				break
			}

			for _, timeFormat := range timeFormats {
				if parseVal, parseErr := time.Parse(timeFormat, ret); parseErr == nil && parseVal.Unix() > 0 {
					ret = fmt.Sprintf("%d", parseVal.Unix())
					break
				}
			}
		case TYPE_DATETIME:
			// check if unixtimestamp
			if timestamp, err := strconv.ParseFloat(ret, 64); err == nil {
				ret = time.Unix(int64(timestamp), 0).Format(time.RFC3339)
				break
			}

			for _, timeFormat := range timeFormats {
				if parseVal, parseErr := time.Parse(timeFormat, ret); parseErr == nil && parseVal.Unix() > 0 {
					ret = parseVal.Format(time.RFC3339)

					break
				}
			}
		default:
			panic(fmt.Errorf(`label type "%s" not supported`, *m.Type))
		}
	}

	return
}

func (m *MetricPathConfig) ParseValue(val interface{}) (ret *float64) {
	if m.Type == nil {
		switch v := val.(type) {
		case int64:
			val := float64(v)
			ret = &val
		case float64:
			ret = &v
		case string:
			if timestamp, err := strconv.ParseFloat(v, 64); err == nil {
				ret = &timestamp
				break
			}
		}
	}

	switch *m.Type {
	case TYPE_TIMESTAMP:
		switch v := val.(type) {
		case int64:
			val := float64(v)
			ret = &val
		case float64:
			ret = &v
		case string:
			// check if string is timestamp
			if timestamp, err := strconv.ParseFloat(v, 64); err == nil {
				ret = &timestamp
				break
			}

			// check date formats
			for _, timeFormat := range timeFormats {
				if parseVal, parseErr := time.Parse(timeFormat, v); parseErr == nil && parseVal.Unix() > 0 {
					val := float64(parseVal.Unix())
					ret = &val
					break
				}
			}

		}
	default:
		panic(fmt.Errorf(`value type "%s" not supported`, *m.Type))
	}

	return
}

func (m *ConfigMetrics) IsValidObject(object unstructured.Unstructured) bool {
	// no filters = is valid
	if len(m.Filters) == 0 {
		return true
	}

	for _, filterConfig := range m.Filters {
		if results, err := filterConfig._path.FindResults(object.Object); err == nil {
			if len(results) == 1 && len(results[0]) == 1 {
				val := results[0][0].Interface()
				if val == nil {
					// no value, object is filtered
					return false
				}

				// convert to string and check if there is a value
				value := fmt.Sprintf("%v", val)
				if value == "" {
					return false
				}
			} else {
				return false
			}
		}
	}

	return true
}

func compileJsonPath(path string) (*jsonpath.JSONPath, error) {
	path = strings.TrimSpace(path)

	jsonPathString, err := get.RelaxedJSONPathExpression(path)
	if err != nil {
		return nil, fmt.Errorf(`unable to build JSONpath "%s": %w`, jsonPathString, err)
	}

	ret := jsonpath.New("jsonpath")
	ret.AllowMissingKeys(true)
	if err := ret.Parse(jsonPathString); err != nil {
		return nil, fmt.Errorf(`unable to parse JSONpath "%s": %w`, jsonPathString, err)
	}

	return ret, nil
}
