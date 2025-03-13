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
	"net/http"
	"sort"

	"github.com/gocolly/colly"
	prowjobv1 "sigs.k8s.io/prow/pkg/apis/prowjobs/v1"
)

const jobName = "ci-kubernetes-e2e-azure-scalability"

func main() {
	prowjobsURL := "https://prow.k8s.io/prowjobs.js?omit=annotations,labels,decoration_config,pod_spec"
	resp, err := http.Get(prowjobsURL)
	if err != nil {
		fmt.Println("Error fetching Prow jobs:", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	prowJobs := &prowjobv1.ProwJobList{}
	if err := json.Unmarshal(body, prowJobs); err != nil {
		fmt.Println("Error unmarshalling Prow jobs:", err)
		return
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
		fmt.Println("No successful Prow jobs found for job name:", jobName)
		return
	}

	latestBuildId := capzProwJobs[0].Status.BuildID
	fmt.Println("Latest Build ID:", latestBuildId)

	c := colly.NewCollector()

	// Find and visit all links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		fmt.Println("Link found:", e.Attr("href"))
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	c.Visit("https://gcsweb.k8s.io/gcs/kubernetes-ci-logs/logs/ci-kubernetes-e2e-azure-scalability/" + latestBuildId + "/artifacts/")
}
