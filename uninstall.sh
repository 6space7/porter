#!/usr/bin/env bash
set -euo pipefail

PORTER_CONFIG_DIR="/etc/porter"
PORTER_DATA_DIR="/var/lib/porter"
PORTER_BINARY="/usr/local/bin/porter"
PORTER_SERVICE_NAME="porter"
PORTER_SERVICE_FILE="/etc/systemd/system/${PORTER_SERVICE_NAME}.service"
PURGE=false

usage() {
	cat <<'EOF'
Usage: sudo ./uninstall.sh [--purge]

Removes the porter systemd service and /usr/local/bin/porter.

Options:
  --purge   Also remove /etc/porter and /var/lib/porter data.
  -h, --help
            Show this help.
EOF
}

require_root() {
	if [[ "${EUID}" -ne 0 ]]; then
		echo "porter uninstaller must run as root" >&2
		exit 1
	fi
}

parse_args() {
	while [[ $# -gt 0 ]]; do
		case "$1" in
			--purge)
				PURGE=true
				;;
			-h|--help)
				usage
				exit 0
				;;
			*)
				echo "unknown argument: $1" >&2
				usage >&2
				exit 1
				;;
		esac
		shift
	done
}

stop_service() {
	if command -v systemctl >/dev/null 2>&1; then
		systemctl disable --now "${PORTER_SERVICE_NAME}" >/dev/null 2>&1 || true
	fi
}

remove_systemd_unit() {
	rm -f "${PORTER_SERVICE_FILE}"
	if command -v systemctl >/dev/null 2>&1; then
		systemctl daemon-reload >/dev/null 2>&1 || true
	fi
}

remove_binary() {
	rm -f "${PORTER_BINARY}"
}

purge_data() {
	rm -rf "${PORTER_CONFIG_DIR}" "${PORTER_DATA_DIR}"
}

print_summary() {
	echo "porter service and binary removed."
	if [[ "${PURGE}" == "true" ]]; then
		echo "Configuration and data directories were purged."
	else
		echo "Configuration and data were kept:"
		echo "  ${PORTER_CONFIG_DIR}"
		echo "  ${PORTER_DATA_DIR}"
		echo "Run with --purge to remove those directories."
	fi
	echo "Docker containers, images, volumes, and networks were not removed."
}

main() {
	require_root
	parse_args "$@"
	stop_service
	remove_systemd_unit
	remove_binary
	if [[ "${PURGE}" == "true" ]]; then
		purge_data
	fi
	print_summary
}

main "$@"
