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

	if [ -f "$shell_rc" ]; then
		if grep -q "$marker" "$shell_rc"; then
			echo "✓ Shell integration already in $shell_rc"
			return
		fi
	fi

	cat >> "$shell_rc" <<'EOF'

# welp shell integration
# Automatically run welp on command errors
__welp_handler() {
    local exit_code=$?
    if [ $exit_code -ne 0 ]; then
        local cmd="${BASH_COMMAND}"
        if [[ ! "$cmd" =~ ^welp|^echo|^read ]]; then
            echo "❌ Command failed: $cmd"
            echo "🔍 Analyzing error..."
            sleep 0.5
        fi
    fi
    return $exit_code
}
trap '__welp_handler' ERR
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
echo "Test:"
echo "  pip install nonexistent-package 2>&1 | welp"
