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
)

const (
	TYPE_TIMESTAMP = "timestamp"
	TYPE_DATETIME  = "datetime"
)

type (
	Config struct {
		Metrics []*ConfigMetrics `yaml:"metrics"`
	}

	ConfigMetrics struct {
		Metric   *MetricConfig               `yaml:"metric"`
		Resource schema.GroupVersionResource `yaml:"resource"`
		Selector *metav1.LabelSelector       `yaml:"selector"`
		Filters  []string                    `yaml:"filters"`
		_filters []*jsonpath.JSONPath
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
	for _, path := range m.Filters {
		if path, err := compileJsonPath(path); err == nil {
			m._filters = append(m._filters, path)
		} else {
			return err
		}
	}

	return nil
}

func (m *ConfigMetrics) KubeMetaListOptions() metav1.ListOptions {
	opts := metav1.ListOptions{}
	if m.Selector != nil {
		opts.LabelSelector = metav1.FormatLabelSelector(m.Selector)
	}

	return opts
}

func (m *ConfigMetrics) LabelSelectorString() string {
	return metav1.FormatLabelSelector(m.Selector)
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

	fmt.Println(val)
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

	for _, filter := range m._filters {
		if results, err := filter.FindResults(object.Object); err == nil {
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

	ret := jsonpath.New("jsonpath")
	ret.AllowMissingKeys(true)
	if err := ret.Parse(path); err != nil {
		return nil, fmt.Errorf(`unable to compile JSONpath "%s": %w`, path, err)
	}

	return ret, nil
}
