#!/usr/bin/env bash
set -eu

image_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

cat >"$tmp/ssh" <<'EOF'
#!/usr/bin/env bash
printf '%s\0' "$@" >"$CAPTURE"
exit 23
EOF
chmod +x "$tmp/ssh"

run_case() {
    local workdir=$1 status=0 arg
    PATH="$tmp:$PATH" CAPTURE="$tmp/args" TARGET_HOST=vm1 TARGET_USER=candidate \
        REMOTE_WORKDIR="$workdir" "$image_dir/remote-shell.sh" || status=$?
    [ "$status" -eq 23 ]

    args=()
    while IFS= read -r -d '' arg; do
        args+=("$arg")
    done <"$tmp/args"
}

run_case /srv/app
expected=(-tt -o BatchMode=yes -l candidate -- vm1 \
    'cd -- '\''/srv/app'\'' && exec "${SHELL:-/bin/sh}" -l')
[ "${#args[@]}" -eq "${#expected[@]}" ]
for i in "${!expected[@]}"; do
    [ "${args[$i]}" = "${expected[$i]}" ]
done

pwned="$tmp/pwned"
run_case "/missing'; touch '$pwned'; #"
bash -c "${args[7]}" 2>/dev/null || true
[ ! -e "$pwned" ]

jq -e '
  .["terminal.integrated.defaultProfile.linux"] == "Sandbox VM" and
  .["terminal.integrated.profiles.linux"]["Sandbox VM"].path == "/usr/local/bin/p3-ide-remote-shell"
' "$image_dir/settings.json" >/dev/null
