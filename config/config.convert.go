package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	CONVERT_TOTIMESTAMP = "totimestamp"
	CONVERT_TODATETIME  = "todatetime"
	CONVERT_TOUPPER     = "toupper"
	CONVERT_TOLOWER     = "tolower"
	CONVERT_TRIM        = "trim"
)

func (m *ConfigMetricJsonPath) DoConvertValue(v string) (ret *float64) {
	if val, err := strconv.ParseFloat(v, 64); err == nil {
		ret = &val
	}

convertLoop:
	for _, method := range m.Convert {
		switch strings.ToLower(*method) {
		case CONVERT_TOTIMESTAMP:
			// check if string is timestamp
			if ret != nil {
				// already possible unix timestamp
				continue convertLoop
			}

			// check date formats
			for _, timeFormat := range timeFormats {
				if parseVal, parseErr := time.Parse(timeFormat, v); parseErr == nil && parseVal.Unix() > 0 {
					val := float64(parseVal.Unix())
					ret = &val
					continue convertLoop
				}
			}

			// conversion failed, to not use value
			ret = nil

		default:
			panic(fmt.Errorf(`value conversion "%s" not supported`, *method))
		}
	}

	return
}

func (m *ConfigMetricJsonPath) DoConvertLabel(val string) (ret string) {
	ret = val

convertLoop:
	for _, method := range m.Convert {
		switch strings.ToLower(*method) {
		case CONVERT_TOTIMESTAMP:
			// check if unixtimestamp
			if _, err := strconv.ParseFloat(ret, 64); err == nil {
				// already timestamp, keep it
				continue convertLoop
			}

			for _, timeFormat := range timeFormats {
				if parseVal, parseErr := time.Parse(timeFormat, ret); parseErr == nil && parseVal.Unix() > 0 {
					ret = fmt.Sprintf("%d", parseVal.Unix())
					continue convertLoop
				}
			}

			// conversion failed, to not use value
			ret = ""
		case CONVERT_TODATETIME:
			// check if unixtimestamp
			if timestamp, err := strconv.ParseFloat(ret, 64); err == nil {
				ret = time.Unix(int64(timestamp), 0).Format(time.RFC3339)
				continue convertLoop
			}

			for _, timeFormat := range timeFormats {
				if parseVal, parseErr := time.Parse(timeFormat, ret); parseErr == nil && parseVal.Unix() > 0 {
					ret = parseVal.Format(time.RFC3339)
					continue convertLoop
				}
			}

			// conversion failed, to not use value
			ret = ""

		case CONVERT_TOUPPER:
			ret = strings.ToUpper(ret)

		case CONVERT_TOLOWER:
			ret = strings.ToLower(ret)

		case CONVERT_TRIM:
			ret = strings.TrimSpace(ret)

		default:
			panic(fmt.Errorf(`label value conversion "%s" not supported`, *method))
		}
	}

	return
}
