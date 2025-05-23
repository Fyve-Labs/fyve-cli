#!/usr/bin/env bash

set -e

log() {
    echo "[FYVE INSTALLER] $1"
}

error() {
    echo "[FYVE INSTALLER] ERROR: $1" >&2
    exit 1
}

# Detect OS and Architecture
detect_system() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        arm64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    case "$OS" in
        darwin|linux) ;;
        *) error "Unsupported operating system: $OS" ;;
    esac

    log "Detected system: $OS/$ARCH"
}

# Fetch latest release version
get_latest_version() {
    VERSION=$(curl -s https://api.github.com/repos/fyve-labs/fyve-cli/releases/latest | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
    log "Latest version: $VERSION"
}

download_file() {
    local URL=$1
    local OUTPUT=$2

    if command -v curl > /dev/null; then
        log "Downloading using curl"
        curl -sL -o "$OUTPUT" "$URL" || error "curl download failed"
    elif command -v wget > /dev/null; then
        log "Downloading using wget"
        wget -O "$OUTPUT" "$URL" || error "wget download failed"
    else
        error "No download utility available (curl/wget)"
    fi
}

verify_checksum() {
    local ARCHIVE=$1
    log "Downloading checksums"
    download_file "https://github.com/fyve-labs/fyve-cli/releases/download/v$VERSION/checksums.txt" checksums.txt

    if command -v sha256sum > /dev/null; then
        CHECKSUM=$(sha256sum "$ARCHIVE" | awk '{print $1}')
    elif command -v shasum > /dev/null; then
        CHECKSUM=$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')
    else
        log "Warning: Cannot verify checksum. No sha256 utility found."
        return
    fi

    if grep -q "$CHECKSUM.*fyve-cli_$OS-$ARCH.tar.gz" checksums.txt; then
        log "Checksum verified successfully"
    else
        error "Checksum verification failed"
    fi
}

go_install() {
    log "Attempting Go installation method"
    if command -v go > /dev/null; then
        go install github.com/fyve-labs/fyve-cli@latest || error "Go install failed"
        log "Successfully installed via go install"
        exit 0
    else
        error "Go is not installed"
    fi
}

main() {
    detect_system
    get_latest_version

    # Construct download URL
    ARCHIVE="fyve-cli_$OS-$ARCH.tar.gz"
    DOWNLOAD_URL="https://github.com/fyve-labs/fyve-cli/releases/download/v$VERSION/$ARCHIVE"

    # Create temporary directory
    TEMP_DIR=$(mktemp -d "/tmp/fyve-cli-intaller-XXXXXX")
    cd "$TEMP_DIR"

    # Download archive
    log "Downloading $ARCHIVE"
    download_file "$DOWNLOAD_URL" "$ARCHIVE"

    # Verify checksum
    verify_checksum "$ARCHIVE"

    # Extract archive
    log "Extracting archive"
    tar -xzf "$ARCHIVE"

    if [ -n "${GITHUB_ACTIONS-}" ] && [ "${GITHUB_ACTIONS}" == "true" ]; then
        mkdir -p "$HOME/.fyve/bin"
        mv fyve "$HOME/.fyve/bin/fyve"
        echo "$HOME/.fyve/bin" >> $GITHUB_PATH
        log "Added $INSTALL_DIR to \$GITHUB_PATH"
        exit 0
    fi

    log "Moving binary to /usr/local/bin"
    sudo mv fyve /usr/local/bin/fyve
    log "Installation complete!"
}

# Fallback to Go install if no release found
trap 'go_install' ERR

main