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

	"github.com/tidwall/gjson"
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

func main() {
	InitializeLogging()
	logger.Info("starting up")

	CheckIsSet("qbitBaseUrl")
	CheckIsSet("qbitUsername")
	CheckIsSet("qbitPassword")

	// authenticate and get session cookie
	requestUrl := qbitBaseUrl + "/api/v2/auth/login"
	data := url.Values{"username": {qbitUsername}, "password": {qbitPassword}}
	resp, err := client.PostForm(requestUrl, data)
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
	logger.Debug("successfully authenticated to qbit")

	// get app version for debugging purposes
	requestUrl = qbitBaseUrl + "/api/v2/app/version"
	resp, err = client.Get(requestUrl)
	if err != nil {
		logger.Error("failed to get version from qbit", "status_code", resp.StatusCode)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	logger.Debug("retrieved qbit API version", "version", string(body))

	// start infinite loop here to check for bad peers
mainLoop:
	for {
		time.Sleep(3 * time.Second)
		// get list of active torrents
		requestUrl = qbitBaseUrl + "/api/v2/torrents/info?filter=active"
		resp, err = client.Get(requestUrl)
		if err != nil {
			logger.Error("failed to get active torrents", "error", err.Error(), "status_code", resp.StatusCode)
			os.Exit(1)
		}
		body, _ = io.ReadAll(resp.Body)
		if string(body) == `[]` {
			logger.Info("no active torrents")
			continue mainLoop
		}

		// parse active torrents - get hash where state=uploading
		if !gjson.ValidBytes(body) {
			logger.Error("invalid json response when attempting to get active torrents")
			os.Exit(1)
		}
		activeTorrents := gjson.ParseBytes(body)
		uploadingHashes := activeTorrents.Get(`#(state=="uploading")#.hash`)
		if len(uploadingHashes.Array()) == 0 {
			logger.Info("no torrents are uploading")
			continue mainLoop
		}

		// get info on uploading torrents
		var badPeers []string
		// ? would be good practice for concurrency here - could probably run each in a goroutine
		for _, hash := range uploadingHashes.Array() {
			requestUrl = fmt.Sprintf("%s/api/v2/sync/torrentPeers?hash=%s", qbitBaseUrl, hash)
			resp, err = client.Get(requestUrl)
			if err != nil {
				logger.Error("failed to get torrent by hash", "error", err.Error(), "status_code", resp.StatusCode)
				os.Exit(1)
			}
			body, _ = io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				logger.Error("qbit API returned error when trying to get torrent by hash", "status_code", resp.StatusCode,
					"response", string(body), "hash", hash.Str,
				)
				os.Exit(1)
			}
			if !gjson.ValidBytes(body) {
				logger.Error("invalid json response when attempting to parse uploading torrent", "hash", hash.Str)
				os.Exit(1)
			}

			// iterate over each peer and find the ip/port of peers using the TS0008 peer ID
			details := gjson.ParseBytes(body)
			peers := details.Get(`peers`)
			peers.ForEach(func(key, value gjson.Result) bool {
				if value.Get("peer_id_client").Str == "-TS0008-" {
					badPeers = append(badPeers, key.String())
					return false // stop iteration once match is found
				}
				return true
			})
		}
		if len(badPeers) > 0 {
			logger.Debug("found bad peers", "peers", badPeers)
		} else {
			logger.Debug("no bad peers found")
			continue mainLoop
		}

		// ban them
		requestUrl = qbitBaseUrl + "/api/v2/transfer/banPeers"
		data = url.Values{"peers": {strings.Join(badPeers[:], "|")}} // produces {"peers": "1.2.3.4:55|5.6.7.8:99"}
		resp, err = client.PostForm(requestUrl, data)
		if err != nil {
			logger.Error("failed to ban bad peers", "error", err.Error()) // don't include status code because docs say that it always returns a 200 OK
			os.Exit(1)                                                    // https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)#ban-peers
		}

		body, _ = io.ReadAll(resp.Body)
		if len(body) != 0 {
			logger.Error("invalid response when trying to ban peers", "peers", badPeers, "response", string(body))
			os.Exit(1)
		} else {
			logger.Info("banned some peers", "peers", badPeers)
		}
	}
}
