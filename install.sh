#!/usr/bin/env bash
set -euo pipefail

PORTER_CONFIG_DIR="/etc/porter"
PORTER_DATA_DIR="/var/lib/porter"
PORTER_WORKSPACE_DIR="${PORTER_DATA_DIR}/work"
PORTER_ENV_FILE="${PORTER_CONFIG_DIR}/porter.env"
PORTER_MASTER_KEY="${PORTER_CONFIG_DIR}/master.key"
PORTER_INITIAL_PASSWORD="${PORTER_CONFIG_DIR}/initial-password"
PORTER_BINARY="/usr/local/bin/porter"
PORTER_SERVICE="/etc/systemd/system/porter.service"
PORTER_ADMIN_EMAIL="${PORTER_ADMIN_EMAIL:-admin@porter.local}"

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
	apt-get install -y ca-certificates curl git openssl tar
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

go_version_ok() {
	if ! command -v go >/dev/null 2>&1; then
		return 1
	fi

	local version major minor
	version="$(go env GOVERSION 2>/dev/null || true)"
	version="${version#go}"
	major="${version%%.*}"
	minor="${version#*.}"
	minor="${minor%%.*}"
	if [[ -z "${major}" || -z "${minor}" ]]; then
		return 1
	fi
	if (( major > 1 )); then
		return 0
	fi
	[[ "${major}" == "1" && "${minor}" -ge 25 ]]
}

go_arch() {
	case "$(uname -m)" in
		x86_64|amd64)
			printf 'amd64'
			;;
		aarch64|arm64)
			printf 'arm64'
			;;
		*)
			echo "unsupported Go architecture: $(uname -m)" >&2
			exit 1
			;;
	esac
}

install_go_if_missing() {
	if go_version_ok; then
		return
	fi

	local version arch archive url tmp
	version="${PORTER_GO_VERSION:-}"
	if [[ -z "${version}" ]]; then
		version="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"
	fi
	arch="$(go_arch)"
	archive="${version}.linux-${arch}.tar.gz"
	url="https://go.dev/dl/${archive}"
	tmp="/tmp/${archive}"

	curl -fsSL "${url}" -o "${tmp}"
	rm -rf /usr/local/go
	tar -C /usr/local -xzf "${tmp}"
	ln -sf /usr/local/go/bin/go /usr/local/bin/go
	ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
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

new_password() {
	openssl rand -base64 24 | tr -d '\n'
}

detect_public_ip() {
	if [[ -n "${PORTER_PUBLIC_IP:-}" ]]; then
		printf '%s' "${PORTER_PUBLIC_IP}"
		return
	fi
	curl -fsS https://api.ipify.org || true
}

platform_domain() {
	local public_ip="$1"
	if [[ -z "${public_ip}" ]]; then
		return
	fi
	printf 'porter.%s.sslip.io' "${public_ip//./-}"
}

write_env_file() {
	local bootstrap_password=""
	local public_ip=""
	local domain=""

	if [[ -f "${PORTER_ENV_FILE}" ]]; then
		chmod 0600 "${PORTER_ENV_FILE}"
		return
	fi

	bootstrap_password="$(new_password)"
	public_ip="$(detect_public_ip)"
	domain="$(platform_domain "${public_ip}")"

	{
		echo "PORTER_HTTP_ADDR=:8080"
		echo "PORTER_DATABASE_PATH=${PORTER_DATA_DIR}/porter.db"
		echo "PORTER_WORKSPACE_PATH=${PORTER_WORKSPACE_DIR}"
		echo "PORTER_CADDY_ASK_URL=http://host.docker.internal:8080/api/v1/caddy/ask"
		echo "PORTER_MANAGE_CADDY=true"
		echo "PORTER_PLATFORM_UPSTREAM=host.docker.internal:8080"
		echo "PORTER_MASTER_KEY_PATH=${PORTER_MASTER_KEY}"
		echo "PORTER_BOOTSTRAP_ADMIN_EMAIL=${PORTER_ADMIN_EMAIL}"
		echo "PORTER_BOOTSTRAP_ADMIN_PASSWORD_FILE=${PORTER_INITIAL_PASSWORD}"
		if [[ -n "${public_ip}" ]]; then
			echo "PORTER_PUBLIC_IP=${public_ip}"
		fi
		if [[ -n "${domain}" ]]; then
			echo "PORTER_PLATFORM_DOMAIN=${domain}"
		fi
	} > "${PORTER_ENV_FILE}"
	chmod 0600 "${PORTER_ENV_FILE}"

	echo "${bootstrap_password}" > "${PORTER_INITIAL_PASSWORD}"
	chmod 0600 "${PORTER_INITIAL_PASSWORD}"
}

build_binary() {
	if ! command -v go >/dev/null 2>&1; then
		echo "Go is required to build porter from source and could not be installed." >&2
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
	systemctl enable porter
	systemctl restart porter
}

print_summary() {
	local platform_url=""
	if [[ -r "${PORTER_ENV_FILE}" ]]; then
		# shellcheck disable=SC1090
		source "${PORTER_ENV_FILE}"
		if [[ -n "${PORTER_PLATFORM_DOMAIN:-}" ]]; then
			platform_url="https://${PORTER_PLATFORM_DOMAIN}"
		fi
	fi

	echo
	echo "porter is installed."
	if [[ -n "${platform_url}" ]]; then
		echo "Dashboard/API: ${platform_url}"
	else
		echo "API: http://127.0.0.1:8080"
	fi
	if [[ -f "${PORTER_INITIAL_PASSWORD}" ]]; then
		echo "Initial admin email: ${PORTER_ADMIN_EMAIL}"
		echo "Initial admin password:"
		cat "${PORTER_INITIAL_PASSWORD}"
		echo
		echo "The password is stored once at ${PORTER_INITIAL_PASSWORD}; remove that file after saving the password."
	else
		echo "Existing install detected; no initial password was generated."
	fi
}

main() {
	require_root
	require_supported_platform
	install_base_packages
	install_docker_if_missing
	ensure_docker_running
	install_go_if_missing
	ensure_directories
	ensure_master_key
	write_env_file
	build_binary
	write_systemd_unit
	start_service
	print_summary
}

main "$@"
