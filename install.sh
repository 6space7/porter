#!/usr/bin/env bash
set -euo pipefail

PORTER_CONFIG_DIR="/etc/porter"
PORTER_DATA_DIR="/var/lib/porter"
PORTER_WORKSPACE_DIR="${PORTER_DATA_DIR}/work"
PORTER_ENV_FILE="${PORTER_CONFIG_DIR}/porter.env"
PORTER_MASTER_KEY="${PORTER_CONFIG_DIR}/master.key"
PORTER_BINARY="/usr/local/bin/porter"
PORTER_SERVICE="/etc/systemd/system/porter.service"

require_root() {
	if [[ "${EUID}" -ne 0 ]]; then
		echo "porter installer must run as root" >&2
		exit 1
	fi
}

require_supported_platform() {
	if [[ ! -r /etc/os-release ]]; then
		echo "unsupported OS: /etc/os-release is missing" >&2
		exit 1
	fi

	# shellcheck disable=SC1091
	source /etc/os-release
	local distro="${ID:-}"
	local like="${ID_LIKE:-}"
	if [[ "${distro}" != "ubuntu" && "${distro}" != "debian" && "${like}" != *"debian"* ]]; then
		echo "unsupported OS: porter Phase 1 installer supports Debian or Ubuntu" >&2
		exit 1
	fi

	local machine
	machine="$(uname -m)"
	case "${machine}" in
		x86_64|amd64|aarch64|arm64)
			;;
		*)
			echo "unsupported architecture: ${machine}" >&2
			exit 1
			;;
	esac
}

install_base_packages() {
	apt-get update
	apt-get install -y ca-certificates curl git openssl
}

install_docker_if_missing() {
	if command -v docker >/dev/null 2>&1; then
		return
	fi

	curl -fsSL https://get.docker.com -o /tmp/get-docker.sh
	sh /tmp/get-docker.sh
}

ensure_docker_running() {
	systemctl enable --now docker
}

ensure_directories() {
	install -d -m 0755 "${PORTER_CONFIG_DIR}"
	install -d -m 0755 "${PORTER_DATA_DIR}"
	install -d -m 0755 "${PORTER_WORKSPACE_DIR}"
}

ensure_master_key() {
	if [[ -f "${PORTER_MASTER_KEY}" ]]; then
		chmod 0600 "${PORTER_MASTER_KEY}"
		return
	fi

	openssl rand -hex 32 > "${PORTER_MASTER_KEY}"
	chmod 0600 "${PORTER_MASTER_KEY}"
}

new_token() {
	printf 'ptr_%s' "$(openssl rand -base64 32 | tr -d '+/=' | head -c 43)"
}

sha256_hex() {
	printf '%s' "$1" | sha256sum | awk '{print $1}'
}

detect_public_ip() {
	if [[ -n "${PORTER_PUBLIC_IP:-}" ]]; then
		printf '%s' "${PORTER_PUBLIC_IP}"
		return
	fi
	curl -fsS https://api.ipify.org || true
}

write_env_file() {
	local bootstrap_token=""
	local bootstrap_hash=""
	local public_ip=""

	if [[ -f "${PORTER_ENV_FILE}" ]]; then
		chmod 0600 "${PORTER_ENV_FILE}"
		return
	fi

	bootstrap_token="$(new_token)"
	bootstrap_hash="$(sha256_hex "${bootstrap_token}")"
	public_ip="$(detect_public_ip)"

	{
		echo "PORTER_HTTP_ADDR=:8080"
		echo "PORTER_DATABASE_PATH=${PORTER_DATA_DIR}/porter.db"
		echo "PORTER_WORKSPACE_PATH=${PORTER_WORKSPACE_DIR}"
		echo "PORTER_CADDY_ASK_URL=http://127.0.0.1:8080/api/v1/caddy/ask"
		echo "PORTER_MANAGE_CADDY=true"
		echo "PORTER_BOOTSTRAP_TOKEN_HASH=${bootstrap_hash}"
		echo "PORTER_MASTER_KEY_PATH=${PORTER_MASTER_KEY}"
		if [[ -n "${public_ip}" ]]; then
			echo "PORTER_PUBLIC_IP=${public_ip}"
		fi
	} > "${PORTER_ENV_FILE}"
	chmod 0600 "${PORTER_ENV_FILE}"

	echo "${bootstrap_token}" > "${PORTER_CONFIG_DIR}/initial-token"
	chmod 0600 "${PORTER_CONFIG_DIR}/initial-token"
}

build_binary() {
	if ! command -v go >/dev/null 2>&1; then
		echo "Go is required to build porter from source. Install Go 1.25 or newer, then rerun this installer." >&2
		exit 1
	fi

	local script_dir
	script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	(cd "${script_dir}" && go build -o "${PORTER_BINARY}" ./cmd/server)
}

write_systemd_unit() {
	cat > "${PORTER_SERVICE}" <<EOF
[Unit]
Description=porter self-hosted PaaS
Wants=network-online.target
After=network-online.target docker.service
Requires=docker.service

[Service]
Type=simple
EnvironmentFile=${PORTER_ENV_FILE}
WorkingDirectory=${PORTER_DATA_DIR}
ExecStart=${PORTER_BINARY}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
}

start_service() {
	systemctl daemon-reload
	systemctl enable --now porter
}

print_summary() {
	local token_file="${PORTER_CONFIG_DIR}/initial-token"
	echo
	echo "porter is installed."
	echo "API: http://127.0.0.1:8080"
	if [[ -f "${token_file}" ]]; then
		echo "Initial bearer token:"
		cat "${token_file}"
		echo
		echo "The token is stored once at ${token_file}; remove that file after saving the token."
	else
		echo "Existing install detected; no initial token was generated."
	fi
}

main() {
	require_root
	require_supported_platform
	install_base_packages
	install_docker_if_missing
	ensure_docker_running
	ensure_directories
	ensure_master_key
	write_env_file
	build_binary
	write_systemd_unit
	start_service
	print_summary
}

main "$@"
