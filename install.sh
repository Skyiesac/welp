#!/bin/bash
set -e

INSTALL_DIR="${HOME}/.local/bin"
TARGET="${INSTALL_DIR}/welp"

echo "Installing welp..."

cd "$(dirname "$0")"
go build -o welp

mkdir -p "$INSTALL_DIR"

if [[ -e "$TARGET" && ! -w "$TARGET" ]]; then
	echo "✗ Cannot write to ${TARGET} (file is not owned by you)."
	echo "  Fix ownership, then re-run this script:"
	echo "    sudo chown \"\${USER}:\${USER}\" \"${TARGET}\""
	echo "  Do not install welp with sudo."
	exit 1
fi

cp welp "$TARGET"
chmod +x "$TARGET"

echo "✓ welp installed to ${TARGET}"
echo "  Run: welp setup"

# Add shell integration
add_shell_integration() {
	local shell_rc="$1"
	local marker="# welp shell integration"

	if [ -f "$shell_rc" ] && grep -q "$marker" "$shell_rc"; then
		tmp_file="$(mktemp)"
		awk '
			$0 == "# welp shell integration" { skip=1; next }
			skip && $0 == "trap __welp_err_trap ERR" { skip=0; next }
			skip && $0 == "trap '\''__welp_handler'\'' ERR" { skip=0; next }
			!skip { print }
		' "$shell_rc" > "$tmp_file"
		mv "$tmp_file" "$shell_rc"
	fi

	cat >> "$shell_rc" <<'EOF'

# welp shell integration
# Automatically run welp on command errors
__welp_err_trap() {
    local status=$?
    [[ $status -eq 130 || $status -eq 143 ]] && return $status
    echo "$BASH_COMMAND" | timeout --foreground 60 welp 2>/dev/null || true
}
trap __welp_err_trap ERR
EOF

	echo "✓ Added shell integration to $shell_rc"
}

if [ -n "$BASH_VERSION" ]; then
	add_shell_integration ~/.bashrc
elif [ -n "$ZSH_VERSION" ]; then
	add_shell_integration ~/.zshrc
fi

echo ""
echo "Installation complete."
echo ""
echo "Next:"
echo "  welp setup"
echo ""
