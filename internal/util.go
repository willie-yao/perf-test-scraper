package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	prowjobv1 "sigs.k8s.io/prow/pkg/apis/prowjobs/v1"
)

func getFileNameFromURL(url string) string {
	urlParts := strings.Split(url, "/")
	return urlParts[len(urlParts)-1]
}

func getJSONFromURL(url string) ([]byte, error) {
	// Get json data from the url
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	jsonBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return jsonBody, nil
}

func parseBuildIDFromURL(url string) string {
	urlTrim := strings.TrimPrefix(url, "https://storage.googleapis.com/kubernetes-ci-logs/logs/ci-kubernetes-e2e-azure-scalability/")
	urlTrimParts := strings.Split(urlTrim, "/")
	buildID := urlTrimParts[0]

	return buildID
}

func GetLatestBuildID(jobName string) (string, error) {
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
