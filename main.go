package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	// Auth
	qbitBaseUrl  = os.Getenv("qbitBaseUrl")
	qbitUsername = os.Getenv("qbitUsername")
	qbitPassword = os.Getenv("qbitPassword")

	// Global HTTP client with a cookie jar
	jar, _ = cookiejar.New(nil)
	client = &http.Client{Jar: jar}
)

func CheckIsSet(envName string) {
	env := os.Getenv(envName)
	if env != "" {
		logger.Debug(envName + " is set")
	} else {
		logger.Error(envName + " is not set")
		os.Exit(1)
	}
}

// RetryRequest retries an HTTP request if it fails, up to maxRetries times.
func RetryRequest(req *http.Request, maxRetries int, delay time.Duration) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := range maxRetries {
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// Close response body before retrying
		if resp != nil {
			resp.Body.Close()
		}

		fmt.Printf("Request failed (attempt %d/%d), retrying in %v...\n", i+1, maxRetries, delay)
		time.Sleep(delay)
	}
	return nil, fmt.Errorf("request failed after %d attempts: %v", maxRetries, err)
}

func main() {
	InitializeLogging()

	CheckIsSet("qbitBaseUrl")
	CheckIsSet("qbitUsername")
	CheckIsSet("qbitPassword")

	// Step 1: Authenticate and get session cookie
	requestUrl := qbitBaseUrl + "/api/v2/auth/login"
	data := url.Values{"username": {qbitUsername}, "password": {qbitPassword}}
	req, _ := http.NewRequest("POST", requestUrl, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := RetryRequest(req, 3, 2*time.Second)
	if err != nil {
		fmt.Println("Error logging in:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("failed to authenticate to qbit", "status_code", resp.StatusCode)
		return
	}

	logger.Debug("successfully authenticated to qbit", "status_code", resp.StatusCode)

	// Step 2: Use the authenticated session to fetch torrent list
	requestUrl = qbitBaseUrl + "/api/v2/app/version"
	req, _ = http.NewRequest("GET", requestUrl, nil)

	resp, err = RetryRequest(req, 3, 2*time.Second)
	if err != nil {
		logger.Error("failed to get version from qbit", "status_code", resp.StatusCode)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	logger.Info(string(body))
}
