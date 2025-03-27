/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/pkg/errors"
	prowjobv1 "sigs.k8s.io/prow/pkg/apis/prowjobs/v1"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const jobName = "ci-kubernetes-e2e-azure-scalability"

var jsonData = map[string][]byte{}

func recordMetrics() {
	go func() {
		for {
			opsProcessed.Inc()
			time.Sleep(2 * time.Second)
		}
	}()
}

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_processed_ops_total",
		Help: "The total number of processed events",
	})
)

func getLatestBuildId() (string, error) {
	prowjobsURL := "https://prow.k8s.io/prowjobs.js?omit=annotations,labels,decoration_config,pod_spec"
	resp, err := http.Get(prowjobsURL)
	if err != nil {
		fmt.Println("Error fetching Prow jobs:", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return "", err
	}

	prowJobs := &prowjobv1.ProwJobList{}
	if err := json.Unmarshal(body, prowJobs); err != nil {
		fmt.Println("Error unmarshalling Prow jobs:", err)
		return "", err
	}

	capzProwJobs := []prowjobv1.ProwJob{}

	for _, job := range prowJobs.Items {
		if job.Spec.Job == jobName && job.Status.State == prowjobv1.SuccessState {
			capzProwJobs = append(capzProwJobs, job)
		}
	}

	// sort capzProwJobs by build id
	sort.Slice(capzProwJobs, func(i, j int) bool {
		return capzProwJobs[i].Status.BuildID > capzProwJobs[j].Status.BuildID
	})

	if len(capzProwJobs) == 0 {
		err = fmt.Errorf("no successful Prow jobs found for job name: %s", jobName)
		return "", err
	}

	latestBuildId := capzProwJobs[0].Status.BuildID
	fmt.Println("Latest Build ID:", latestBuildId)
	return latestBuildId, nil
}

func addJsonMetricToPrometheus(raw []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		panic(err)
	}

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
				Subsystem: "PodStartupLatency",
				Name:      metricName,
				Help:      metricName,
			},
			[]string{"perc"},
		)
		prometheus.MustRegister(podStartup)

		for k, v := range dataItem {
			if value, ok := v.(float64); ok {
				podStartup.WithLabelValues(k).Set(value)
			}
		}
	}

	return nil
}

func main() {
	c := colly.NewCollector()

	// Channel to communicate the latest build ID
	latestBuildIdChan := make(chan string)

	// Goroutine to fetch the latest build ID every hour
	go func() {
		for {
			fmt.Println("Fetching latest build ID...")
			latestBuildId, err := getLatestBuildId()
			if err != nil {
				log.Println("Error getting latest build ID:", err)
			} else {
				latestBuildIdChan <- latestBuildId
			}
			time.Sleep(1 * time.Hour)
		}
	}()

	// Start a goroutine to listen for the latest build ID and trigger scraping
	go func() {
		for latestBuildId := range latestBuildIdChan {
			c.Visit("https://gcsweb.k8s.io/gcs/kubernetes-ci-logs/logs/ci-kubernetes-e2e-azure-scalability/" + latestBuildId + "/artifacts/")
		}
	}()

	// Find and visit all links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// fmt.Println("Link found:", link)
		if strings.Contains(link, "/PodStartupLatency_PodStartupLatency_load") {
			// fmt.Println("Found PodStartupLatency link:", link)

			// Get json data from the link
			resp, err := http.Get(link)
			if err != nil {
				fmt.Println("Error fetching JSON data:", err)
				return
			}
			defer resp.Body.Close()

			jsonBody, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Println("Error reading JSON response body:", err)
				return
			}

			if err := addJsonMetricToPrometheus(jsonBody); err != nil {
				fmt.Println("Error adding JSON metric to Prometheus:", err)
			}

		}
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(":8080", nil))
}
