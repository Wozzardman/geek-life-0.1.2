#!/usr/bin/env bash

set -euo pipefail

SKIP_PATH_UPDATE=0
USER_BIN_DIR="${HOME}/.local/bin"

for arg in "$@"; do
    case "$arg" in
        --skip-path-update)
            SKIP_PATH_UPDATE=1
            ;;
        --bin-dir=*)
            USER_BIN_DIR="${arg#*=}"
            ;;
        *)
            echo "[geek-life] Unknown argument: $arg" >&2
            echo "Usage: ./install.sh [--skip-path-update] [--bin-dir=/path/to/bin]" >&2
            exit 1
            ;;
    esac
done

write_step() {
    echo "[geek-life] $1"
}

ensure_go_installed() {
    if ! command -v go >/dev/null 2>&1; then
        echo "[geek-life] Go is not installed or not in PATH." >&2
        echo "[geek-life] On CachyOS/Arch, install with: sudo pacman -S go" >&2
        exit 1
    fi
}

update_path_if_needed() {
    local dir="$1"

    if [[ ":$PATH:" == *":$dir:"* ]]; then
        write_step "$dir is already in PATH"
        return
    fi

    if [[ "$SKIP_PATH_UPDATE" -eq 1 ]]; then
        write_step "Skipping PATH update as requested"
        return
    fi

    local shell_name
    shell_name="$(basename "${SHELL:-}")"
    local profile_file=""

    case "$shell_name" in
        bash)
            profile_file="$HOME/.bashrc"
            ;;
        zsh)
            profile_file="$HOME/.zshrc"
            ;;
        *)
            if [[ -f "$HOME/.profile" ]]; then
                profile_file="$HOME/.profile"
            elif [[ -f "$HOME/.bashrc" ]]; then
                profile_file="$HOME/.bashrc"
            elif [[ -f "$HOME/.zshrc" ]]; then
                profile_file="$HOME/.zshrc"
            else
                profile_file="$HOME/.profile"
            fi
            ;;
    esac

    local export_line="export PATH=\"$dir:\$PATH\""
    if [[ ! -f "$profile_file" ]]; then
        touch "$profile_file"
    fi

    if ! grep -Fq "$export_line" "$profile_file"; then
        printf '\n%s\n' "$export_line" >> "$profile_file"
        write_step "Added $dir to PATH in $profile_file"
    else
        write_step "$dir is already configured in $profile_file"
    fi

    export PATH="$dir:$PATH"
}

main() {
    local script_dir output_file target_file

    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    output_file="$script_dir/geek-life"
    target_file="$USER_BIN_DIR/geek-life"

    ensure_go_installed

    write_step "Building geek-life"
    (
        cd "$script_dir"
        go build -o "$output_file" ./app
    )

    write_step "Installing to $target_file"
    mkdir -p "$USER_BIN_DIR"
    install -m 0755 "$output_file" "$target_file"

    update_path_if_needed "$USER_BIN_DIR"

    write_step "Install complete. Run: geek-life"
}

main "$@"