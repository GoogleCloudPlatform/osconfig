#!/usr/bin/env bash
#
# install_lang_mgrs.sh - Install language package managers, toolchains,
# and sample packages/projects on Debian 13 for testing SCALIBR extractors.
#
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "Error: This script must be run as root (or with sudo)." >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive

echo "===> [1/10] Updating package lists and installing core dependencies..."
apt-get update || true
apt-get install -y --no-install-recommends \
  ca-certificates \
  curl \
  wget \
  gnupg \
  git \
  build-essential \
  pkg-config \
  unzip \
  zip || true

echo "===> [2/10] Installing package managers from Debian repositories..."
debian_packages=(
  python3 python3-pip python3-venv python3-setuptools python3-wheel
  ruby ruby-dev
  nodejs npm
  default-jdk maven gradle
  rustc cargo
  php-cli composer
)
apt-get install -y --no-install-recommends "${debian_packages[@]}" || {
  echo "Batch apt install failed, attempting individual package installation..."
  for pkg in "${debian_packages[@]}"; do
    apt-get install -y --no-install-recommends "$pkg" || echo "Warning: failed to install apt package $pkg" >&2
  done
}

echo "===> [3/10] Installing latest Go from go.dev..."
(
  GO_VERSION=$(curl -s https://go.dev/VERSION?m=text 2>/dev/null | head -n 1 || echo "go1.23.2")
  echo "Downloading Go version: ${GO_VERSION}..."
  curl -fsSL "https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm -f /tmp/go.tar.gz

  ln -sf /usr/local/go/bin/go /usr/local/bin/go
  ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

  if ! grep -q '/usr/local/go/bin' ~/.bashrc 2>/dev/null; then
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
  fi
  export PATH=$PATH:/usr/local/go/bin
) || echo "Warning: failed to install Go from go.dev" >&2

echo "===> [4/10] Installing Python uv..."
if ! command -v uv &>/dev/null; then
  curl -LsSf https://astral.sh/uv/install.sh | env CARGO_DIST_FORCE_INSTALLERS=shell UV_INSTALL_DIR=/usr/local/bin sh || true
fi

echo "===> [5/10] Installing cargo-auditable (for rust/cargoauditable)..."
if command -v cargo &>/dev/null; then
  cargo install cargo-auditable --root /usr/local || true
fi

echo "===> [6/10] Installing .NET SDK (for dotnet/nugetcpm)..."
if ! command -v dotnet &>/dev/null; then
  (
    curl -sSL https://dot.net/v1/dotnet-install.sh -o /tmp/dotnet-install.sh && \
    chmod +x /tmp/dotnet-install.sh && \
    /tmp/dotnet-install.sh --channel 8.0 --install-dir /usr/share/dotnet && \
    ln -sf /usr/share/dotnet/dotnet /usr/local/bin/dotnet && \
    rm -f /tmp/dotnet-install.sh
  ) || echo "Warning: failed to install .NET SDK" >&2
fi

echo "===> [7/10] Installing Dart SDK (for dart/pubspec)..."
if ! command -v dart &>/dev/null; then
  (
    install -m 0755 -d /etc/apt/keyrings && \
    wget -qO- https://dl-ssl.google.com/linux/linux_signing_key.pub | gpg --dearmor --yes -o /etc/apt/keyrings/dart.gpg && \
    echo 'deb [signed-by=/etc/apt/keyrings/dart.gpg] https://storage.googleapis.com/download.dartlang.org/linux/debian stable main' > /etc/apt/sources.list.d/dart_stable.list && \
    apt-get update && \
    apt-get install -y dart && \
    ln -sf /usr/lib/dart/bin/dart /usr/local/bin/dart
  ) || echo "Warning: failed to install Dart SDK" >&2
fi

echo "===> [8/10] Installing Swift toolchain (for swift/packageresolved)..."
if ! command -v swift &>/dev/null; then
  (
    apt-get install -y --no-install-recommends \
      binutils \
      libcurl4 \
      libedit2 \
      libgcc-s1 \
      libgomp1 \
      libsqlite3-0 \
      libstdc++6 \
      libxml2 \
      zlib1g || true

    curl -L https://swift-server.github.io/swiftly/swiftly-install.sh -o /tmp/swiftly-install.sh
    chmod +x /tmp/swiftly-install.sh
    SWIFTLY_HOME_DIR=/usr/local/swiftly /tmp/swiftly-install.sh -y --platform linux || true
    rm -f /tmp/swiftly-install.sh

    if [ -f /usr/local/swiftly/bin/swift ]; then
      ln -sf /usr/local/swiftly/bin/swift /usr/local/bin/swift
    fi
  ) || echo "Warning: failed to install Swift toolchain" >&2
fi

echo "===> [9/10] Installing sample packages & creating sample projects for all extractors..."
mkdir -p /opt/samples || true

# 1. python/wheelegg (global site-packages wheels)
pip3 install --break-system-packages requests flask || true

# 2. python/uvlock
if command -v uv &>/dev/null; then
  mkdir -p /opt/samples/python-uv || true
  (cd /opt/samples/python-uv && uv init --no-workspace && uv add requests) || true
fi

# 3. ruby/gemspec
gem install rake sinatra --no-document || true

# 4. javascript/packagejson (global npm packages)
npm install -g --force lodash express chalk || true

# 5. java/pomxml
mkdir -p /opt/samples/java-maven || true
cat << 'EOF' > /opt/samples/java-maven/pom.xml || true
<project xmlns="http://maven.apache.org/POM/4.0.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>sample-app</artifactId>
  <version>1.0.0</version>
  <dependencies>
    <dependency>
      <groupId>org.slf4j</groupId>
      <artifactId>slf4j-api</artifactId>
      <version>2.0.9</version>
    </dependency>
  </dependencies>
</project>
EOF

# 6. java/gradlelockfile
mkdir -p /opt/samples/java-gradle/gradle/dependency-locks || true
cat << 'EOF' > /opt/samples/java-gradle/gradle/dependency-locks/compileClasspath.lockfile || true
# Gradle dependency lockfile
org.slf4j:slf4j-api:2.0.9=compileClasspath
empty=
EOF

# 7. go/binary (binary with embedded modules) & go/gomod
GOBIN=/usr/local/bin go install golang.org/x/tools/cmd/goimports@latest || true
mkdir -p /opt/samples/go || true
(cd /opt/samples/go && go mod init example.com/sample && go get github.com/gin-gonic/gin@v1.9.1) || true

# 8. rust/cargolock & rust/cargoauditable
mkdir -p /opt/samples/rust || true
(cd /opt/samples/rust && cargo init --bin sample && cargo add serde && cargo build) || true
if command -v cargo-auditable &>/dev/null; then
  (cd /opt/samples/rust && cargo auditable build --release && cp target/release/sample /usr/local/bin/sample-rust-auditable) || true
fi

# 9. php/composerlock
mkdir -p /opt/samples/php || true
(cd /opt/samples/php && composer require guzzlehttp/guzzle:^7.0 --no-interaction) || true

# 10. dart/pubspec
if command -v dart &>/dev/null; then
  mkdir -p /opt/samples/dart || true
  (cd /opt/samples/dart && dart create -t console sample && cd sample && dart pub add http && dart pub get) || true
fi

# 11. swift/packageresolved
mkdir -p /opt/samples/swift || true
cat << 'EOF' > /opt/samples/swift/Package.resolved || true
{
  "pins": [
    {
      "identity": "alamofire",
      "kind": "remoteSourceControl",
      "location": "https://github.com/Alamofire/Alamofire.git",
      "state": {
        "revision": "f455c297587245a0c55a48e65c8ff03401fa25a4",
        "version": "5.8.1"
      }
    }
  ],
  "version": 2
}
EOF

# 12. dotnet/nugetcpm
mkdir -p /opt/samples/dotnet || true
cat << 'EOF' > /opt/samples/dotnet/Directory.Packages.props || true
<Project>
  <PropertyGroup>
    <ManagePackageVersionsCentrally>true</ManagePackageVersionsCentrally>
  </PropertyGroup>
  <ItemGroup>
    <PackageVersion Include="Newtonsoft.Json" Version="13.0.3" />
  </ItemGroup>
</Project>
EOF

echo "===> [10/10] Verification summary of installed package managers:"
echo "------------------------------------------------------------"
for cmd in python3 pip3 uv ruby gem node npm java mvn gradle go cargo cargo-auditable php composer dotnet dart swift; do
  if command -v "$cmd" &>/dev/null; then
    ver=$("$cmd" --version 2>&1 | head -n 1 || true)
    printf "%-18s: INSTALLED (%s)\n" "$cmd" "$ver"
  else
    printf "%-18s: NOT FOUND\n" "$cmd"
  fi
done
echo "------------------------------------------------------------"
echo "Setup completed successfully."
