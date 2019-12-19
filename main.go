package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

// This is for unmarshalling the PRs API call response.
type pullRequestFetchedData []struct {
	Number         int    `json:"number"`
	MergeCommitSha string `json:"merge_commit_sha"`
	StatusesURL    string `json:"statuses_url"`
}

// This is for unmarshalling statuses API call response.
type pullRequestStatuses []struct {
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
}

// This is the shape for our PRs objects.
type githubPRData struct {
	PullRequest  int
	CommitSHA    string
	ChecksPassed bool
	// Status       string
}

// Config
var (
	organization = "ograu"      // "giantswarm"
	repo         = "prsfetcher" // "happa"
	// Create a Bearer string by appending string access token
	bearer = "Bearer " + os.Getenv("PRSFETCHER_GITHUB_TOKEN")
	// See: https://developer.github.com/v3/pulls/
	pullRequestsEndpoint = fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", organization, repo)
)

// It returns a list of commit sha 's of all open and green PR's
func main() {
	// PRs API call
	body, err := getAPICall(pullRequestsEndpoint)

	if err != nil {
		fmt.Printf("The HTTP request failed with error %s\n", err)
		panic(err)
	}

	// Our array of PR data object/struct fetched
	var data pullRequestFetchedData

	errData := json.Unmarshal(body, &data)
	if errData != nil {
		panic(err)
	}

	// This is what we will return, an slice/array with PR data objects
	var allPRsData []githubPRData

	for _, v := range data {
		// Statuses url returns an array of objects. The first object has a key success that can hold succes/failure
		prsData := githubPRData{
			PullRequest:  v.Number,
			CommitSHA:    v.MergeCommitSha,
			ChecksPassed: true,
		}

		// Statuses API call
		body, err := getAPICall(v.StatusesURL)
		if err != nil {
			fmt.Printf("Error reading the response %s\n", err)
			panic(err)
		}

		var data pullRequestStatuses

		errData := json.Unmarshal(body, &data)
		if errData != nil {
			panic(err)
		}

		// Last one seems to be the first one
		if data[0].State == "success" {
			allPRsData = append(allPRsData, prsData)
		}

	}

	// All PRS opened and green
	for _, v := range allPRsData {
		fmt.Println("data", v)
	}
}

func getAPICall(endpoint string) ([]byte, error) {
	// Create a new request using http
	req, err := http.NewRequest("GET", endpoint, nil)

	// add authorization header to the req
	req.Header.Add("Authorization", bearer)

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)

	// resp, err := http.Get(endpoint)
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var body = []byte(bodyBytes)
	return body, nil
}

// https://developer.github.com/v3/issues/comments/#create-a-comment
func createPRComment(PRNumber string, URL string) ([]byte, error) {
	createPRCommentEndpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%s/comments", organization, repo, PRNumber)

	type Comment struct {
		Body string `json:"body"`
	}
	comment := Comment{"This branch is deployed here: " + URL}
	jsonStr, err := json.Marshal(comment)

	req, err := http.NewRequest("POST", createPRCommentEndpoint, bytes.NewBuffer(jsonStr))
	req.Header.Add("Authorization", bearer)
	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.comfort-fade-preview+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))

	return body, nil
}
