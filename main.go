package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
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

func ClearBannedIPs() {
	requestUrl := qbitBaseUrl + "/api/v2/app/setPreferences"

	// payload needs to look like `json=<urlencodedpayload>`
	prefsData := map[string]string{"banned_IPs": ""}
	payload, _ := json.Marshal(prefsData)

	resp, err := client.PostForm(requestUrl, url.Values{"json": {string(payload)}})
	if err != nil {
		logger.Error("returned error when trying to send request to clear banned IPs from qbit config", "error", err.Error())
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		logger.Error("invalid response when trying to clear list of banned peers", "response", string(body), "status_code", resp.StatusCode)
	} else {
		logger.Info("cleared banned peer list", "wait_time", "6h")
	}

	resp.Body.Close()
}

func CheckForBadPeers() {
	// get list of active torrents
	requestUrl := qbitBaseUrl + "/api/v2/torrents/info?filter=active"
	resp, err := client.Get(requestUrl)
	if err != nil {
		logger.Error("failed to get active torrents", "error", err.Error(), "status_code", resp.StatusCode)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) == `[]` {
		logger.Debug("no active torrents")
		return
	}

	// parse active torrents - get hash where state=uploading
	if !gjson.ValidBytes(body) {
		logger.Error("invalid json response when attempting to get active torrents")
		os.Exit(1)
	}
	activeTorrents := gjson.ParseBytes(body)
	uploadingHashes := activeTorrents.Get(`#(state=="uploading")#.hash`)
	if len(uploadingHashes.Array()) == 0 {
		logger.Debug("torrents are active but none are uploading")
		return
	}

	// get info on uploading torrents
	var badPeers []peerInfo
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, hash := range uploadingHashes.Array() {
		wg.Add(1)
		go func(hash gjson.Result) {
			defer wg.Done()
			requestUrl := fmt.Sprintf("%s/api/v2/sync/torrentPeers?hash=%s", qbitBaseUrl, hash)
			resp, err := client.Get(requestUrl)
			if err != nil {
				logger.Error("failed to get torrent by hash", "error", err.Error(), "status_code", resp.StatusCode, "hash", hash.Str)
				os.Exit(1)
			}
			body, _ := io.ReadAll(resp.Body)
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

			// iterate over each peer and find the ip/port of peers using a blacklisted peer ID
			details := gjson.ParseBytes(body)
			peers := details.Get(`peers`)
			peers.ForEach(func(key, value gjson.Result) bool {
				peerId := value.Get("peer_id_client").Str
				if peerId == "-TS0008-" || // torrentstorm (stremio)
					peerId == "Unknown" || // not sure what these are but they seem sus
					strings.HasPrefix(peerId, "-WW00") || // webtorrent
					strings.HasPrefix(peerId, "-Lr") || // movie downloader/movietvshow
					strings.HasPrefix(peerId, "-LT11") { // elementum

					mu.Lock()
					badPeers = append(badPeers, peerInfo{Addr: key.String(), Hash: hash.Str, Id: peerId})
					mu.Unlock()
				}
				return true
			})
		}(hash)
	}
	wg.Wait()

	if len(badPeers) > 0 {
		logger.Debug("found bad peers", "peers", badPeers)
	} else {
		logger.Debug("torrents are uploading but no bad peers found")
		return
	}

	// ban them
	requestUrl = qbitBaseUrl + "/api/v2/transfer/banPeers"
	addrs := make([]string, len(badPeers))
	for i, p := range badPeers {
		addrs[i] = p.Addr
	}
	data := url.Values{"peers": {strings.Join(addrs[:], "|")}} // produces {"peers": "1.2.3.4:55|5.6.7.8:99"}
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
		return
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

	// retry a few times in case qbit hasn't started up yet.
	// if the program dies and restarts four or five times before it successfully authenticates, it'll clog up the logs
	var resp *http.Response
	var err error
	for i := 1; i <= 5; i++ {
		resp, err = client.PostForm(requestUrl, data)
		if err == nil {
			logger.Debug("successfully authenticated to qbit")
			break
		}
		logger.Debug("qbit authentication request returned an error", "error", err.Error())
		time.Sleep(2 * time.Second) // wait before retrying
	}
	if err != nil { // if can't connect after 5 attempts, exit
		logger.Error("unable to authenticate to qbit", "error", err.Error())
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

	// get app version for debugging purposes
	requestUrl = qbitBaseUrl + "/api/v2/app/version"
	resp, err = client.Get(requestUrl)
	if err != nil {
		logger.Error("failed to get version from qbit", "status_code", resp.StatusCode)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	logger.Info("retrieved qbit API version", "version", string(body))

	ClearBannedIPs()

	// reset the banned IPs every so often
	go func() {
		for {
			time.Sleep(6 * time.Hour)
			ClearBannedIPs()
		}
	}()

	// start infinite loop here to check for bad peers
	logger.Info("watching for bad peers")
	for {
		CheckForBadPeers()
		time.Sleep(10 * time.Second)
	}
}
