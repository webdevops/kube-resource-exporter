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
		Metric          *MetricConfig               `yaml:"metric"`
		Resource        schema.GroupVersionResource `yaml:"resource"`
		Selector        *metav1.LabelSelector       `yaml:"selector"`
		Filters         []string                    `yaml:"filters"`
		compiledFilters []*jsonpath.JSONPath
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
		Path     string `yaml:"jsonPath" json:"jsonPath"`
		jsonPath *jsonpath.JSONPath
		Type     *string `yaml:"type"`
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
	if m.jsonPath == nil {
		jsonPathString := strings.TrimSpace(m.Path)

		path := jsonpath.New("jsonpath")
		path.AllowMissingKeys(true)
		if err := path.Parse(jsonPathString); err != nil {
			return nil, err
		}

		m.jsonPath = path
	}

	return m.jsonPath, nil
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
		if v, ok := val.(float64); ok {
			return &v
		}
	}

	switch *m.Type {
	case TYPE_TIMESTAMP:
		switch v := val.(type) {
		case float64:
			ret = &v
		case string:
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

	// compile filters
	if len(m.compiledFilters) == 0 {
		for _, filter := range m.Filters {
			jsonPathString := strings.TrimSpace(filter)

			path := jsonpath.New("jsonpath")
			path.AllowMissingKeys(true)
			if err := path.Parse(jsonPathString); err != nil {
				panic(err)
			}

			m.compiledFilters = append(m.compiledFilters, path)
		}
	}

	for _, filter := range m.compiledFilters {
		if results, err := filter.FindResults(object.Object); err == nil {
			if len(results) == 1 && len(results[0]) == 1 {
				val := results[0][0].Interface()

				if val == nil {
					// no value, object is filtered
					return false
				}

				// convert to string and check if there is a value
				value := fmt.Sprintf("%v", val)
				if value != "" {
					return false
				}
			} else {
				return false
			}
		}
	}

	return true
}
