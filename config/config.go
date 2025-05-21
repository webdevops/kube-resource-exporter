package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/jsonpath"
	"k8s.io/kubectl/pkg/cmd/get"
)

const (
	KUBE_SELECTOR_ERROR = "<error>"
	KUBE_SELECTOR_NONE  = "<none>"
)

type (
	Config struct {
		Resources []*ConfigResource `yaml:"resources"`
	}

	ConfigResource struct {
		*schema.GroupVersionResource `yaml:",inline"`

		Selector  *metav1.LabelSelector `yaml:"selector"`
		_selector string

		Metrics []*ConfigMetric `yaml:"metrics"`
	}

	ConfigMetric struct {
		Name   string                        `yaml:"name"`
		Help   string                        `yaml:"help"`
		Value  *ConfigMetricValue            `yaml:"value"`
		Labels map[string]*ConfigMetricLabel `yaml:"labels"`

		Filters []*ConfigMetricFilter `yaml:"filters"`
	}

	ConfigMetricValue struct {
		*ConfigMetricJsonPath `yaml:",inline"`
		Value                 *float64 `yaml:"value"`
	}

	ConfigMetricLabel struct {
		*ConfigMetricJsonPath `yaml:",inline"`
		Value                 string `yaml:"value"`
	}

	ConfigMetricJsonPath struct {
		Path    string `yaml:"jsonPath" json:"jsonPath"`
		_path   *jsonpath.JSONPath
		Convert []*string `yaml:"convert"`
	}

	ConfigMetricFilter struct {
		Path  string `yaml:"jsonPath" json:"jsonPath"`
		_path *jsonpath.JSONPath

		Regex  string `yaml:"regex"`
		_regex *regexp.Regexp
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
	for _, row := range m.Resources {
		err := row.Compile()
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *ConfigResource) Compile() error {
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}

	if m.Resource == "" {
		return fmt.Errorf("resource is required")
	}

	// selector
	if m.Selector != nil {
		selector := metav1.FormatLabelSelector(m.Selector)
		if strings.EqualFold(selector, KUBE_SELECTOR_ERROR) {
			return fmt.Errorf(`unable to compile Kubernetes selector for resource "%s/%s/%s"`, m.Group, m.Version, m.Resource)
		}

		if !strings.EqualFold(selector, KUBE_SELECTOR_NONE) {
			m._selector = selector
		}
	}

	for _, row := range m.Metrics {
		err := row.Compile()
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *ConfigMetric) Compile() error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}

	// value path
	if m.Value.ConfigMetricJsonPath != nil && m.Value.Path != "" {
		if path, err := compileJsonPath(m.Value.Path); err == nil {
			m.Value._path = path
		} else {
			return err
		}
	}

	// labels path
	for _, labelConfig := range m.Labels {
		if labelConfig.ConfigMetricJsonPath != nil && labelConfig.Path != "" {
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

		// compile jsonPath
		if path, err := compileJsonPath(filterConfig.Path); err == nil {
			filterConfig._path = path
		} else {
			return err
		}

		// compile regex
		if filterConfig.Regex != "" {
			filterRegex, err := regexp.Compile(filterConfig.Regex)
			if err != nil {
				return err
			}

			filterConfig._regex = filterRegex
		}
	}

	return nil
}

func (m *ConfigResource) KubeMetaListOptions() metav1.ListOptions {
	opts := metav1.ListOptions{}
	if m._selector != "" {
		opts.LabelSelector = m._selector
	}

	return opts
}

func (m *ConfigMetricJsonPath) JsonPath() *jsonpath.JSONPath {
	if m == nil {
		return nil
	}

	return m._path
}

func (m *ConfigMetricJsonPath) ParseLabel(val interface{}) (ret string) {
	// convert type
	switch v := val.(type) {
	case float64:
		ret = fmt.Sprintf("%f", v)
	case int64:
		ret = fmt.Sprintf("%d", v)
	case string:
		ret = v
	case bool:
		if v {
			ret = "true"
		} else {
			ret = "false"
		}
	}

	return m.DoConvertLabel(ret)
}

func (m *ConfigMetricJsonPath) ParseValue(val interface{}) (ret *float64) {
	valueString := ""
	switch v := val.(type) {
	case float64:
		valueString = fmt.Sprintf("%f", v)
	case int64:
		valueString = fmt.Sprintf("%d", v)
	case string:
		valueString = v
	case bool:
		if v {
			valueString = "1"
		} else {
			valueString = "0"
		}
	}

	return m.DoConvertValue(valueString)
}

func (m *ConfigMetric) IsValidObject(object unstructured.Unstructured) bool {
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

				// check regexp
				if filterConfig._regex != nil {
					if !filterConfig._regex.MatchString(value) {
						return false
					}
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
