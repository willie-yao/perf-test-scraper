package internal

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterPodStartupMetricsToProm(link string, clusterName string) error {
	buildID := parseBuildIDFromURL(link)
	fileName := getFileNameFromURL(link)
	data, err := getJSONFromURL(link)
	if err != nil {
		return err
	}

	fileNameParts := strings.Split(fileName, "_")
	name := strings.Join(fileNameParts[:len(fileNameParts)-2], "_")
	// fmt.Println("Name:", name)

	dataItems, found := data["dataItems"].([]interface{})
	if !found {
		return errors.Errorf("dataItems not found or invalid format")
	}

	for _, e := range dataItems {
		item, ok := e.(map[string]interface{})
		if !ok {
			return errors.Errorf("Invalid data item format")
		}

		metricName, _ := item["labels"].(map[string]interface{})["Metric"].(string)
		if metricName == "" {
			return errors.Errorf("Metric name not found")
		}

		dataItem, ok := item["data"].(map[string]interface{})
		if !ok {
			return errors.Errorf("data not found or invalid format")
		}

		podStartup := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "capz",
				Subsystem: name,
				Name:      metricName,
				Help:      metricName,
			},
			[]string{"perc", "cluster", "buildID"},
		)
		prometheus.MustRegister(podStartup)

		for k, v := range dataItem {
			if value, ok := v.(float64); ok {
				podStartup.WithLabelValues(k, clusterName, buildID).Set(value)
			}
		}
	}

	return nil
}
