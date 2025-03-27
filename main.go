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
	"path"
	"sort"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/pkg/errors"
	prowjobv1 "sigs.k8s.io/prow/pkg/apis/prowjobs/v1"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const jobName = "ci-kubernetes-e2e-azure-scalability"

var (
	clusterName string
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

func addJsonMetricToPrometheus(raw []byte, fileName string) error {

	fileNameParts := strings.Split(fileName, "_")
	name := strings.Join(fileNameParts[:len(fileNameParts)-2], "_")
	// fmt.Println("Name:", name)

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
				Subsystem: name,
				Name:      metricName,
				Help:      metricName,
			},
			[]string{"perc", "cluster"},
		)
		prometheus.MustRegister(podStartup)

		for k, v := range dataItem {
			if value, ok := v.(float64); ok {
				podStartup.WithLabelValues(k, clusterName).Set(value)
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

		fmt.Println("Link:", link)
		if strings.HasSuffix(link, "artifacts/clusters/") {
			c.Visit("https://gcsweb.k8s.io" + link)
		}

		if strings.Contains(link, "artifacts/clusters/") {
			base := path.Base(link)
			if strings.HasPrefix(base, "capz-") {
				clusterName = base
				fmt.Println("Got cluster name:", clusterName)
			}
		}

		if strings.Contains(link, "PodStartupLatency") {
			// fmt.Println("Found PodStartupLatency link:", link)
			urlParts := strings.Split(link, "/")
			fileName := urlParts[len(urlParts)-1]
			// fmt.Println("File name:", fileName)

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

			if err := addJsonMetricToPrometheus(jsonBody, fileName); err != nil {
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
