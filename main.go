package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
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
type prsData struct {
	PRNumber     int
	CommitSHA    string
	ChecksPassed bool
	// Status       string
}

type commentsFetchedData []struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

type comment struct {
	Body string `json:"body"`
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
	var allGreenPRs []prsData

	for _, v := range data {
		// Statuses url returns an array of objects. The first object has a key success that can hold succes/failure
		prsData := prsData{
			PRNumber:     v.Number,
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
		// TODO uncomment this
		// if data[0].State == "success" {
		allGreenPRs = append(allGreenPRs, prsData)
		// }

	}

	// All PRS opened and green
	for _, v := range allGreenPRs {
		fmt.Println("data", v)
	}

	// TODO Check which ones are not yet deployed and deploy them.
}

// This function looks for the App PR Deployer *comment* in a PR.
func getComment(PRNumber string) (bool, string, string) {
	commentsEndpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%s/comments", organization, repo, PRNumber)
	body, err := getAPICall(commentsEndpoint)

	if err != nil {
		fmt.Printf("Couldn't fetch comments %s\n", err)
		panic(err)
	}

	// Our array of PR comments fetched
	var data commentsFetchedData

	errData := json.Unmarshal(body, &data)
	if errData != nil {
		fmt.Printf("Couldn't unmarshall comments %s\n", err)
		panic(err)
	}

	// Flag for knowing if it has an App PR Deployer comment
	var hasComment bool
	var idComment string
	var bodyComment string

	for _, v := range data {
		if strings.HasPrefix(v.Body, "### App PR Deployer") {
			hasComment = true
			idComment = strconv.Itoa(v.ID)
			bodyComment = v.Body
		}
	}

	// TODO struct?
	return hasComment, idComment, bodyComment
}

// Creates a comment in a PR/issue
// https://developer.github.com/v3/issues/comments/#create-a-comment
func createPRComment(PRNumber string) ([]byte, error) {
	commentsEndpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%s/comments", organization, repo, PRNumber)

	text := `### App PR Deployer
Comment revision: 2

AWS:   Not initialized
Azure: Not initialized
KVM:   Not initialized`

	comment := comment{text}

	jsonStr, err := json.Marshal(comment)

	req, err := http.NewRequest("POST", commentsEndpoint, bytes.NewBuffer(jsonStr))
	req.Header.Add("Authorization", bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.comfort-fade-preview+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	return body, nil
}

// Takes a comment message text and updates it with the current info
func editComment(provider string, statusOrURL string, comment string) string {
	// Split string in lines and  modify them
	slicedComment := strings.Split(comment, "\n")

	if provider == "AWS" {
		slicedComment[3] = "AWS: " + statusOrURL
	}

	if provider == "Azure" {
		slicedComment[4] = "Azure: " + statusOrURL
	}

	if provider == "KVM" {
		slicedComment[5] = "KVM: " + statusOrURL
	}

	var reunitedComment string
	for _, v := range slicedComment {
		reunitedComment += v + "\n"
	}

	fmt.Printf("%v", reunitedComment)

	return reunitedComment
}

// TODO Check which ones are *being* deployed and update comment in PR accordigly.
// This function is called from installations
func updateGithub(PRNumber string, provider string, statusOrURL string) {
	hasComment, idComment, bodyComment := getComment(PRNumber)

	if !hasComment {
		// TODO use the message from edit comment, otherwise the very first update is skipped
		createPRComment(PRNumber)
	} else {
		// PATCH comment
		newComment := editComment(provider, statusOrURL, bodyComment)
		commentsEndpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%s", organization, repo, idComment)
		commentBody := comment{newComment}

		jsonStr, err := json.Marshal(commentBody)

		req, err := http.NewRequest("PATCH", commentsEndpoint, bytes.NewBuffer(jsonStr))
		req.Header.Add("Authorization", bearer)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
	}
}

// Utility function for making get API calls
func getAPICall(endpoint string) ([]byte, error) {
	// Create a new request using http
	req, err := http.NewRequest("GET", endpoint, nil)

	// add authorization header to the req
	req.Header.Add("Authorization", bearer)

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var body = []byte(bodyBytes)
	return body, nil
}
