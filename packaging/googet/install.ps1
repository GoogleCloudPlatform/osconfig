#  Copyright 2018 Google Inc. All Rights Reserved.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

$ErrorActionPreference = 'Stop'

function Set-ServiceConfig {
  # Restart service after 1s, then 2s. Reset error counter after 60s.
  sc.exe failure google_osconfig_agent reset= 60 actions= restart/1000/restart/2000
  # Set dependency and delayed start
  sc.exe config google_osconfig_agent depend= "rpcss" start= delayed-auto
  # Create trigger to start the service on first IP address
  sc.exe triggerinfo google_osconfig_agent start/networkon
}

try {
  if (-not (Get-Service 'google_osconfig_agent' -ErrorAction SilentlyContinue)) {
    New-Service -DisplayName 'Google OSConfig Agent' `
                -Name 'google_osconfig_agent' `
                -BinaryPathName '"C:\Program Files\Google\OSConfig\google_osconfig_agent.exe"' `
                -StartupType Automatic `
                -Description 'Google OSConfig service agent'

    Set-ServiceConfig
    Start-Service google_osconfig_agent -Verbose -ErrorAction Stop
  } 
  else {
    Set-ServiceConfig
    New-Item -Path 'C:\Program Files\Google\OSConfig\osconfig_agent_restart_required' -Force -Type File -ErrorAction SilentlyContinue | Out-Null
  }
}
catch {
  Write-Output $_.InvocationInfo.PositionMessage
  Write-Output "Install failed: $($_.Exception.Message)"
  exit 1
}
  