#!/bin/bash

set -e

# This script uses the qBittorrent API to check if any torrents are uploading to peers advertising a client string of "TorrentStorm 0.0.0.8" and bans those peers.

# Logging function allows user to customize verbosity
log() {
  local message=$1 # Contents of log message
  local level=$2 # Logging level of the log message. 0=DEBUG, 1=INFO, 2=ERROR

  # If the globally defined logging level is configured to view this message, then echo to STDOUT
  if [ "$level" -ge "$logLevel" ]; then
    timestamp=$(date +%Y-%m-%dT%H:%M:%S%z)
    echo "{\"timestamp\": \"$timestamp\", \"log_level\": \"$level\", \"message\": \"$message\"}"
  fi
}

logLevel="${logLevel:-1}" # Default log level of INFO
# Make sure user hasn't set this to an invalid value
if ! echo "$logLevel" | grep -qE '^[0-9]+$'; then
  timestamp=$(date +%Y-%m-%dT%H:%M:%S%z)
  message="logLevel is not a number but it should be"

  echo "{\"timestamp\": \"$timestamp\", \"log_level\": \"2\", \"message\": \"$message\"}"
  exit 1
fi

sleepTime="${sleepTime:-10}" # Sleep for a default of 10 seconds between checks
# Make sure user hasn't set this to an invalid value
if ! echo "$sleepTime" | grep -qE '^[0-9]+$'; then
  log "sleepTime is not a number but it should be" 2
  exit 1
fi

baseUrl="${qbUrl:-"http://localhost:8080"}" # Set default qBittorrent connection string if not set with environment variable
# Get auth cookie
response=$(curl -sS --header "Referer: ${baseUrl}" --data "username=${qbUsername}&password=${qbPassword}" -c cookies.txt ${baseUrl}/api/v2/auth/login)
if [ "$response" != 'Ok.' ]; then
  log "failed to authenticate to qbit" 2
  exit 1
fi

log "validation complete, now monitoring qBittorrent API" 1
while true; do
  # Wait a little between checks
  sleep ${sleepTime}

  # Get all active torrents from API, then grab the hashes of ones that are currently uploading. Stores as a string separated by newline, e.g., "hash1\nhash2"
  response=$(curl -sS --header "Referer: ${baseUrl}" -b cookies.txt ${baseUrl}/api/v2/torrents/info?filter=active | jq -r '.[] | select(.state == "uploading") | .hash')

  # If no torrents are uploading, then stop the script
  if [ "$response" == '[]' ]; then
    log "no torrents are uploading. retrying in ${sleepTime} seconds" 0
    continue
  fi

  # Split string with hashes into an array
  hashArray=()
  while IFS= read -r line; do
    hashArray+=("$line")
  done <<< "$response"

  # For each torrent that is uploading, get the IP of any peer with a client string of "TorrentStorm 0.0.0.8"
  badIPArray=()
  for hash in "${hashArray[@]}"; do
    response=$(curl -sS --header "Referer: ${baseUrl}" -b cookies.txt ${baseUrl}/api/v2/sync/torrentPeers?hash=${hash} | jq -r '.peers[] | select(.client == "TorrentStorm 0.0.0.8") | .ip')
    # The response will include empty strings for peers that aren't TorrentStorm, so we append only non-empty strings to the array
    while IFS= read -r line; do
      if [[ -n "$line" ]]; then
        badIPArray+=("$line")
      fi
    done <<< "$response"  
  done

  if [ ${#badIPArray[@]} -eq 0 ]; then
    log "${#hashArray[@]} torrents are uploading, but no bad peers found. retrying in ${sleepTime} seconds" 0
    continue
  fi

  # Send them to the shadow realm
  banString=$(IFS=\|; echo "${badIPArray[*]}") # Creates a string like "1.2.3.4:55|6.7.8.9:00" for the qBittorrent API
  curl -sS --header "Referer: ${baseUrl}" -b cookies.txt ${baseUrl}/api/v2/transfer/banPeers?${banString}
  log "banned peers: ${badIPArray[*]}" 1
done
