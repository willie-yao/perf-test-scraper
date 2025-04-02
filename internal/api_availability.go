package internal

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// APIAvailability represents the top-level structure of the JSON.
type APIAvailability struct {
	ClusterMetrics ClusterMetrics `json:"clusterMetrics"`
	HostMetrics    []HostMetrics  `json:"hostMetrics"`
}

// ClusterMetrics represents the metrics for the cluster.
type ClusterMetrics struct {
	AvailabilityPercentage   float64 `json:"availabilityPercentage"`
	LongestUnavailablePeriod string  `json:"longestUnavailablePeriod"`
}

// HostMetrics represents the metrics for each host.
type HostMetrics struct {
	IP                       string  `json:"IP"`
	AvailabilityPercentage   float64 `json:"availabilityPercentage"`
	LongestUnavailablePeriod string  `json:"longestUnavailablePeriod"`
}

func RegisterAPIAvailabilityMetricsToProm(link string, clusterName string) error {
	buildID := parseBuildIDFromURL(link)
	fileName := getFileNameFromURL(link)
	jsonBody, err := getJSONFromURL(link)
	if err != nil {
		return err
	}

	fileNameParts := strings.Split(fileName, "_")
	name := strings.Join(fileNameParts[:len(fileNameParts)-2], "_")

	apiAvailability := &APIAvailability{}
	if err := json.Unmarshal(jsonBody, apiAvailability); err != nil {
		return errors.Errorf("Error unmarshalling JSON: %v", err)
	}

	apiServerAvailabilityGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "capz",
			Subsystem: name,
			Name:      "APIServerAvailabilityPercentage",
			Help:      "APIServerAvailabilityPercentage",
		},
		[]string{"cluster", "buildID"},
	)
	prometheus.MustRegister(apiServerAvailabilityGauge)
	apiServerAvailabilityGauge.WithLabelValues(clusterName, buildID).Set(apiAvailability.ClusterMetrics.AvailabilityPercentage)

	return nil
}
