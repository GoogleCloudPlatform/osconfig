#! /bin/bash

version=$(cat packaging/googet/google-osconfig-agent.goospec | sed -nE 's/.*"version":.*"(.+)".*/\1/p')
if [[ $? -ne 0 ]]; then
  echo "could not match version in goospec"
  exit 1
fi

GOOS=windows $GO build -ldflags="-X main.version=${version}" -mod=readonly -o google_osconfig_agent.exe
