# Copyright 2018 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Don't build debuginfo packages.
%global debug_package %{nil}

Name: google-osconfig-agent
Epoch:   1
Version: %{_version}
Release: g1%{?dist}
Summary: Google Compute Engine guest environment.
License: ASL 2.0
Url: https://github.com/GoogleCloudPlatform/osconfig
Source0: %{name}_%{version}.orig.tar.gz

BuildArch: %{_arch}
%if ! 0%{?el6}
BuildRequires: systemd
%endif

%description
Contains the OSConfig agent binary and startup scripts

%prep
%autosetup

%build
GOPATH=%{_gopath} CGO_ENABLED=0 %{_go} build -ldflags="-s -w -X main.version=%{version}-%{release}" -mod=readonly -o google_osconfig_agent

%install
install -d "%{buildroot}/%{_docdir}/%{name}"
cp -r THIRD_PARTY_LICENSES "%buildroot/%_docdir/%name/THIRD_PARTY_LICENSES"

install -d %{buildroot}%{_bindir}
install -d %{buildroot}/var/lib/google_osconfig_agent
install -p -m 0755 google_osconfig_agent %{buildroot}%{_bindir}/google_osconfig_agent
%if 0%{?el6}
install -d %{buildroot}/etc/init
install -p -m 0644 %{name}.conf %{buildroot}/etc/init
%else
install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_presetdir}
install -p -m 0644 %{name}.service %{buildroot}%{_unitdir}
install -p -m 0644 90-%{name}.preset %{buildroot}%{_presetdir}/90-%{name}.preset
%endif

%files
%{_docdir}/%{name}
%defattr(-,root,root,-)
%{_bindir}/google_osconfig_agent
%if 0%{?el6}
/etc/init/%{name}.conf
%else
%{_unitdir}/%{name}.service
%{_presetdir}/90-%{name}.preset
%endif

%post
%if 0%{?el6}
if [ $1 -eq 1 ]; then
  # Start the service on first install
  start -q -n google-osconfig-agent
fi
%else
%systemd_post google-osconfig-agent.service
if [ $1 -eq 1 ]; then
  # Start the service on first install
  systemctl start google-osconfig-agent.service
fi

if [ $1 -eq 2 ]; then
  # If the old directory exists make sure we set the file there.
  [ -e /etc/osconfig ] && touch /etc/osconfig/osconfig_agent_restart_required
  touch /var/lib/google_osconfig_agent/osconfig_agent_restart_required
fi

%preun
%systemd_preun google-osconfig-agent.service

%postun
%systemd_postun google-osconfig-agent.service

%endif
