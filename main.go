package main

import (
	"os"
	"time"
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

	CheckIsSet("qbitBaseUrl")

	logger.Info("hello")

	time.Sleep(6 * time.Second) // wait for qbit to start

	// TODO: auth to qbit and log error if failed
	// curl --header "Referer: ${baseUrl}" ${baseUrl}/api/v2/auth/login
	// ? save cookie or pass some token? will need to submit auth with subsequent requests

	for {
		time.Sleep(3 * time.Second)
		// TODO: get active torrents
		// curl -sS --header "Referer: ${baseUrl}" -b auth.txt ${baseUrl}/api/v2/torrents/info?filter=active
		// TODO: continue if no torrents are active
		// if response == "" {
		// 	continue
		// }
		// TODO: parse active torrents - get hash where state=uploading
		// TODO: add hashes to slice
		// TODO: get key where value.id (? need real path) is -TS0008- (? need real value)
		// response=$(curl -sS --header "Referer: ${baseUrl}" -b auth.txt ${baseUrl}/api/v2/sync/torrentPeers?hash=${hash} | jq -r '.peers | to_entries[] | select(.value.client == "TorrentStorm 0.0.0.8") | .key')
		// TODO: add key to new badHashSlice
		// TODO: create goofy string for ban API request from badHashSlice
		// banString=$(IFS=\|; echo "${badIPArray[*]}") # Creates a string like "1.2.3.4:55|6.7.8.9:00" for the qBittorrent API
		// TODO: ban them
		// curl -sS --header "Referer: ${baseUrl}" -b auth.txt ${baseUrl}/api/v2/transfer/banPeers?${banString}
		// TODO: log banned peers
		// logger.Info("banned some peers", "peers", someJsonArrayWithBannedPeerIPs)
	}
}
