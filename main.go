package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
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

type commentFetchedData struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

type comment struct {
	Body string `json:"body"`
}

type commentID struct {
	ID int `json:"id"`
}

// Config
var (
	organization = "ograu"      // "giantswarm"
	repo         = "prsfetcher" // "happa"
	// Create a Bearer string by appending string access token
	bearer = "Bearer " + os.Getenv("PRSFETCHER_GITHUB_TOKEN")
	// See: https://developer.github.com/v3/pulls/
	pullRequestsEndpoint = fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", organization, repo)
	defaultComment       = `### App PR Deployer
	
AWS:   Not initialized
Azure: Not initialized
KVM:   Not initialized`
)

func main() {
	getPRs()
}

// It returns a list of commit sha 's of all open and green PR's
func getPRs() ([]prsData, error) {
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

	return allGreenPRs, nil
}

func getCommentByID(id int) (string, error) {
	commentsEndpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%v", organization, repo, id)
	body, err := getAPICall(commentsEndpoint)

	if err != nil {
		fmt.Printf("Couldn't fetch comment %s\n", err)
		panic(err)
	}

	// Our array of PR comments fetched
	var data commentFetchedData

	errData := json.Unmarshal(body, &data)
	if errData != nil {
		fmt.Printf("Couldn't unmarshall comment %s\n", err)
		panic(err)
	}

	return data.Body, nil
}

// Creates a comment in a PR/issue
// https://developer.github.com/v3/issues/comments/#create-a-comment
func createPRComment(PRNumber string, commentText string) (int, error) {
	commentsEndpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%s/comments", organization, repo, PRNumber)

	comment := comment{commentText}

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
	// Comment
	var data commentID

	errData := json.Unmarshal(body, &data)
	if errData != nil {
		panic(err)
	}

	fmt.Println(data.ID)

	return data.ID, nil
}

// Takes a comment message text and updates it with the current info
func editComment(provider string, statusOrURL string, comment string) string {
	// Split string in lines and  modify them
	slicedComment := strings.Split(comment, "\n")

	if provider == "AWS" {
		slicedComment[2] = "AWS: " + statusOrURL
	}

	if provider == "Azure" {
		slicedComment[3] = "Azure: " + statusOrURL
	}

	if provider == "KVM" {
		slicedComment[4] = "KVM: " + statusOrURL
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
func updateGithub(PRNumber string, idComment int, provider string, statusOrURL string) {
	// hasComment, idComment, bodyComment := getComment(PRNumber)
	var hasComment bool

	if hasComment {
		// Use default message and modify it
		newComment := editComment(provider, statusOrURL, defaultComment)
		createPRComment(PRNumber, newComment)
	} else {
		// Or PATCH it
		bodyComment, _ := getCommentByID(idComment)
		newComment := editComment(provider, statusOrURL, bodyComment)
		commentsEndpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%v", organization, repo, idComment)
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
