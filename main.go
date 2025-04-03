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
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gocolly/colly"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"sigs.k8s.io/perf-test-scraper/internal"
)

const jobName = "ci-kubernetes-e2e-azure-scalability"

var (
	clusterName string
)

func main() {
	c := colly.NewCollector()

	// Channel to communicate the latest build ID
	latestBuildIdChan := make(chan string)

	// Goroutine to fetch the latest build ID every hour
	go func() {
		for {
			fmt.Println("Fetching latest build ID...")
			latestBuildId, err := internal.GetLatestBuildID(jobName)
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
		url := e.Attr("href")

		if strings.HasSuffix(url, "artifacts/clusters/") {
			c.Visit("https://gcsweb.k8s.io" + url)
		}

		if strings.Contains(url, "artifacts/clusters/") {
			base := path.Base(url)
			if strings.HasPrefix(base, "capz-") {
				clusterName = base
				fmt.Println("Got cluster name:", clusterName)
			}
		}

		if strings.Contains(url, "PodStartupLatency") {
			if err := internal.RegisterPodStartupMetricsToProm(url, clusterName); err != nil {
				fmt.Println("Error registering PodStartupLatency metrics:", err)
			}
		}
		if strings.Contains(url, "APIAvailability") {
			if err := internal.RegisterAPIAvailabilityMetricsToProm(url, clusterName); err != nil {
				fmt.Println("Error registering API availability metrics:", err)
			}
		}
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(":8080", nil))
}
