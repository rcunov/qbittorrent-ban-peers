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
	// auth
	qbitBaseUrl  = os.Getenv("qbitBaseUrl")
	qbitUsername = os.Getenv("qbitUsername")
	qbitPassword = os.Getenv("qbitPassword")

	// global HTTP client with a cookie jar
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

	// authenticate and get session cookie
	requestUrl := qbitBaseUrl + "/api/v2/auth/login"
	data := url.Values{"username": {qbitUsername}, "password": {qbitPassword}}
	req, _ := http.NewRequest(http.MethodPost, requestUrl, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := RetryRequest(req, 3, 2*time.Second)
	if err != nil {
		logger.Error("failed to send qbit authentication request", "error", err.Error())
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	authResponse := string(body)
	if resp.StatusCode != http.StatusOK {
		logger.Error("qbit authentication request returned an error", "status_code", resp.StatusCode)
		os.Exit(1)
	}
	if authResponse != "Ok." {
		logger.Error("invalid credentials for qbit", "response", authResponse)
		os.Exit(1)
	}
	logger.Debug("successfully authenticated to qbit", "status_code", resp.StatusCode, "response", authResponse)

	// get app version for debugging purposes
	requestUrl = qbitBaseUrl + "/api/v2/app/version"
	req, _ = http.NewRequest(http.MethodGet, requestUrl, nil)
	resp, err = RetryRequest(req, 3, 2*time.Second)
	if err != nil {
		logger.Error("failed to get version from qbit", "status_code", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	logger.Debug("retrieved qbit API version", "version", string(body))

	// TODO: start infinite for loop here
	// get list of active torrents
	requestUrl = qbitBaseUrl + "/api/v2/torrents/info?filter=active"
	req, _ = http.NewRequest(http.MethodGet, requestUrl, nil)
	resp, err = RetryRequest(req, 4, 2*time.Second)
	if err != nil {
		logger.Error("failed to get active torrents", "error", err.Error(), "status_code", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	if string(body) == `[]` {
		logger.Info("no active torrents")
		// TODO: restart if no torrents are active
	}

	// TODO: parse active torrents - get hash where state=uploading

	// TODO: add hashes to slice

	// get info on uploading torrents
	requestUrl = qbitBaseUrl + "/api/v2/sync/torrentPeers?hash="
	req, _ = http.NewRequest(http.MethodGet, requestUrl, nil)
	resp, err = RetryRequest(req, 4, 2*time.Second)
	logger.Debug(resp.Status)
	if err != nil {
		logger.Error("failed to get torrent by hash", "error", err.Error(), "status_code", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		logger.Error("qbit API returned error when trying to get torrent by hash", "status_code", resp.StatusCode,
			"response", string(body), "hash", "TODO: hash goes here",
		)
	}

	// TODO: get <ip and port> where peers.<ip and port>.peer_id_client = "-TS0008-"

	// TODO: add peer to badPeerSlice

	// TODO: create goofy string for ban API request from badPeerSlice like "1.2.3.4:55|6.7.8.9:00"

	// TODO: ban them
	// curl -sS --header "Referer: ${baseUrl}" -b auth.txt ${baseUrl}/api/v2/transfer/banPeers?${banString}

	// TODO: log banned peers
	// logger.Info("banned some peers", "peers", someJsonArrayWithBannedPeerIPs)
}
