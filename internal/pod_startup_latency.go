package internal

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// PodStartupLatency represents the top-level structure of the JSON.
type PodStartupLatency struct {
	Version   string     `json:"version"`
	DataItems []DataItem `json:"dataItems"`
}

// DataItem represents a single data item in the "dataItems" array.
type DataItem struct {
	Data   Data   `json:"data"`
	Unit   string `json:"unit"`
	Labels Labels `json:"labels"`
}

// Data represents the metrics data with percentage values.
type Data struct {
	Perc50 float64 `json:"Perc50"`
	Perc90 float64 `json:"Perc90"`
	Perc99 float64 `json:"Perc99"`
}

// Labels represents the metric labels.
type Labels struct {
	Metric string `json:"Metric"`
}

func RegisterPodStartupMetricsToProm(link string, clusterName string) error {
	buildID := parseBuildIDFromURL(link)
	fileName := getFileNameFromURL(link)
	jsonBody, err := getJSONFromURL(link)
	if err != nil {
		return err
	}

	fileNameParts := strings.Split(fileName, "_")
	name := strings.Join(fileNameParts[:len(fileNameParts)-2], "_")

	podStartupMetrics := &PodStartupLatency{}
	if err := json.Unmarshal(jsonBody, podStartupMetrics); err != nil {
		return errors.Errorf("Error unmarshalling JSON: %v", err)
	}

	for _, item := range podStartupMetrics.DataItems {
		metricName := item.Labels.Metric
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

		podStartup.WithLabelValues("Perc50", clusterName, buildID).Set(item.Data.Perc50)
		podStartup.WithLabelValues("Perc90", clusterName, buildID).Set(item.Data.Perc90)
		podStartup.WithLabelValues("Perc99", clusterName, buildID).Set(item.Data.Perc99)
	}

	return nil
}
