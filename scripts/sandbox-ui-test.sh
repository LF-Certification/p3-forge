#!/bin/bash
set -euo pipefail

PREFIX="p3-sandbox-ui-test"
OWNER_LABEL_KEY="io.lf-certification.p3-sandbox-ui-test.owner"
OWNER_LABEL_VALUE="$PREFIX"
OWNER_LABEL="$OWNER_LABEL_KEY=$OWNER_LABEL_VALUE"
RUN_LABEL_KEY="io.lf-certification.p3-sandbox-ui-test.run"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE_FILE="$REPO_ROOT/.sandbox-ui-test-state"

DEFAULT_UI_IMAGE="$PREFIX/ui:local"
DEFAULT_TERMINAL_IMAGE="$PREFIX/terminal:local"
DEFAULT_GENERATOR_IMAGE="$PREFIX/instructions-generator:local"
DEFAULT_SERVER_IMAGE="$PREFIX/instructions-server:local"
DEFAULT_SSH_IMAGE="docker.io/linuxserver/openssh-server@sha256:edbbd662675be4f6a06c76c24f785adf68d7c69156152280842788e85d152a44"
DEFAULT_CHROME_IMAGE="docker.io/zenika/alpine-chrome@sha256:ee10e24217aa27443e6b58da628f3b09ea9b814459915b8b62fe15a555f9692a"

UI_PORT="${SANDBOX_UI_TEST_UI_PORT:-8091}"
INSTRUCTIONS_PORT="${SANDBOX_UI_TEST_INSTRUCTIONS_PORT:-7682}"
TERMINAL_PORT="${SANDBOX_UI_TEST_TERMINAL_PORT:-7685}"
EXPIRY="${SANDBOX_UI_TEST_EXPIRY:-2099-01-01T00:00:00Z}"
OPEN_BROWSER="${SANDBOX_UI_TEST_OPEN:-1}"
UI_IMAGE="${SANDBOX_UI_TEST_UI_IMAGE:-$DEFAULT_UI_IMAGE}"
TERMINAL_IMAGE="${SANDBOX_UI_TEST_TERMINAL_IMAGE:-$DEFAULT_TERMINAL_IMAGE}"
GENERATOR_IMAGE="${SANDBOX_UI_TEST_GENERATOR_IMAGE:-$DEFAULT_GENERATOR_IMAGE}"
SERVER_IMAGE="${SANDBOX_UI_TEST_SERVER_IMAGE:-$DEFAULT_SERVER_IMAGE}"
SSH_IMAGE="${SANDBOX_UI_TEST_SSH_IMAGE:-$DEFAULT_SSH_IMAGE}"
CHROME_IMAGE="${SANDBOX_UI_TEST_CHROME_IMAGE:-$DEFAULT_CHROME_IMAGE}"
RUNTIME_DIR=""
RUN_ID=""

PROTECTED_NAMES=(
    ui-preview
    term-preview
    term-clipboard-review
    term-clipboard-final-uat
    instructions-preview
    ssh-target
    ttyd-preview
)

log() {
    printf '%s\n' "$*"
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

validate_port() {
    local name="${1:-}"
    local value="${2:-}"
    if [[ -z "$name" || ! "$value" =~ ^[0-9]+$ ]] || (( 10#$value < 1 || 10#$value > 65535 )); then
        die "invalid $name: $value"
    fi
}

resource_name() {
    local kind="${1:-}"
    case "$kind" in
        net) printf '%s-net\n' "$PREFIX" ;;
        site) printf '%s-site\n' "$PREFIX" ;;
        creds) printf '%s-creds\n' "$PREFIX" ;;
        ui|terminal|instructions|ssh|generator|chrome-probe|creds-init|site-init|creds-check)
            printf '%s-%s\n' "$PREFIX" "$kind"
            ;;
        *) die "unknown resource kind: $kind" ;;
    esac
}

render_ui_config() {
    local rendered
    rendered="$(jq -cn \
        --arg expiry "$EXPIRY" \
        --arg instructions "http://localhost:$INSTRUCTIONS_PORT" \
        --arg terminal "http://localhost:$TERMINAL_PORT" \
        '{config:{version:"v1",defaultTool:"terminal",expiresAt:$expiry,showTimer:false},tools:[{kind:"instructions",name:"instructions",url:$instructions},{kind:"terminal",name:"terminal",url:$terminal}]}')"
    # The UI image injects this value through a sed replacement whose delimiter
    # and replacement metacharacters are | and &. JSON Unicode escapes preserve
    # the decoded value while keeping those bytes out of the replacement text.
    rendered="${rendered//|/\\u007c}"
    rendered="${rendered//&/\\u0026}"
    printf '%s\n' "$rendered"
}

render_terminal_config() {
    jq -cn --arg host "$(resource_name ssh)" \
        '{targetHost:$host,targetUser:"tux",retryInterval:"5",fontSize:"16"}'
}

is_protected_name() {
    local candidate="$1"
    local protected
    for protected in "${PROTECTED_NAMES[@]}"; do
        [[ "$candidate" == "$protected" ]] && return 0
    done
    return 1
}

assert_removable_name() {
    local name="$1"
    is_protected_name "$name" && die "refusing to remove protected resource: $name"
    [[ "$name" == "$PREFIX-"* ]] || die "refusing to remove resource outside fixture namespace: $name"
}

resource_exists() {
    local kind="$1"
    local name="$2"
    case "$kind" in
        container) docker container inspect "$name" >/dev/null 2>&1 ;;
        network) docker network inspect "$name" >/dev/null 2>&1 ;;
        volume) docker volume inspect "$name" >/dev/null 2>&1 ;;
        *) die "unknown Docker resource kind: $kind" ;;
    esac
}

assert_owned() {
    local name="$1"
    local kind="$2"
    local actual
    case "$kind" in
        container)
            actual="$(docker container inspect --format "{{index .Config.Labels \"$OWNER_LABEL_KEY\"}}" "$name" 2>/dev/null || true)"
            ;;
        network|volume)
            actual="$(docker "$kind" inspect --format "{{index .Labels \"$OWNER_LABEL_KEY\"}}" "$name" 2>/dev/null || true)"
            ;;
        *) die "unknown Docker resource kind: $kind" ;;
    esac
    [[ "$actual" == "$OWNER_LABEL_VALUE" ]] || die "$kind resource '$name' is not owned by $PREFIX"
}

ensure_name_available() {
    local kind="$1"
    local name="$2"
    if resource_exists "$kind" "$name"; then
        assert_owned "$name" "$kind"
        die "$kind resource '$name' already exists"
    fi
}

check_command() {
    command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

resolve_docker_endpoint() {
    local endpoint
    if [[ -n "${DOCKER_HOST:-}" ]]; then
        endpoint="$DOCKER_HOST"
    else
        endpoint="$(docker context inspect --format '{{(index .Endpoints "docker").Host}}' 2>/dev/null || true)"
    fi
    [[ "$endpoint" == unix:///* ]] || die "Docker endpoint must be a local Unix socket (resolved: ${endpoint:-empty})"
    local socket_path="${endpoint#unix://}"
    [[ -S "$socket_path" ]] || die "Docker endpoint is not a Unix socket: $socket_path"
    printf '%s\n' "$endpoint"
}

check_prereqs() {
    check_command docker
    resolve_docker_endpoint >/dev/null
    docker info >/dev/null 2>&1 || die "local Docker daemon is not reachable"
    check_command ssh-keygen
    check_command curl
    check_command jq
    check_command nc
}

validate_settings() {
    validate_port SANDBOX_UI_TEST_UI_PORT "$UI_PORT"
    validate_port SANDBOX_UI_TEST_INSTRUCTIONS_PORT "$INSTRUCTIONS_PORT"
    validate_port SANDBOX_UI_TEST_TERMINAL_PORT "$TERMINAL_PORT"
    [[ "$UI_PORT" != "$INSTRUCTIONS_PORT" && "$UI_PORT" != "$TERMINAL_PORT" && "$INSTRUCTIONS_PORT" != "$TERMINAL_PORT" ]] || \
        die "SANDBOX_UI_TEST ports must be distinct"
    [[ "$OPEN_BROWSER" == 0 || "$OPEN_BROWSER" == 1 ]] || die "invalid SANDBOX_UI_TEST_OPEN: $OPEN_BROWSER"
    render_ui_config >/dev/null
}

assert_port_free() {
    local port="$1"
    local variable="$2"
    if nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
        die "port $port is already in use; choose another value for $variable"
    fi
}

create_runtime() {
    local base="${TMPDIR:-/tmp}"
    RUNTIME_DIR="$(mktemp -d "$base/$PREFIX.XXXXXX")"
    RUNTIME_DIR="$(cd -P -- "$RUNTIME_DIR" && pwd -P)"
    chmod 700 "$RUNTIME_DIR"
    printf '%s\n' "$RUNTIME_DIR" > "$STATE_FILE"
    chmod 600 "$STATE_FILE"
}

canonical_runtime_path() {
    local candidate="$1"
    local allow_missing="${2:-0}"
    local canonical_tmp canonical_candidate candidate_parent candidate_name
    canonical_tmp="$(cd -P -- "${TMPDIR:-/tmp}" && pwd -P)" || die "cannot resolve temporary directory"
    candidate_name="$(basename "$candidate")"
    [[ "$candidate_name" == "$PREFIX."* ]] || die "runtime directory has an unsafe name"
    [[ ! -L "$candidate" ]] || die "runtime directory is a symlink"
    if [[ -e "$candidate" ]]; then
        [[ -d "$candidate" ]] || die "runtime directory is unsafe"
        canonical_candidate="$(cd -P -- "$candidate" && pwd -P)" || die "cannot resolve runtime directory"
    else
        [[ "$allow_missing" == 1 ]] || die "runtime directory is missing"
        candidate_parent="$(dirname "$candidate")"
        [[ -d "$candidate_parent" ]] || die "runtime directory parent is missing"
        candidate_parent="$(cd -P -- "$candidate_parent" && pwd -P)" || die "cannot resolve runtime directory parent"
        canonical_candidate="$candidate_parent/$candidate_name"
    fi
    [[ "$canonical_candidate" != "$canonical_tmp" && "$(dirname "$canonical_candidate")" == "$canonical_tmp" ]] || \
        die "runtime directory is outside the fixture temporary namespace"
    printf '%s\n' "$canonical_candidate"
}

load_runtime() {
    [[ -f "$STATE_FILE" && ! -L "$STATE_FILE" ]] || die "runtime state is missing; run 'up' first"
    [[ "$(wc -l < "$STATE_FILE" | tr -d ' ')" == 1 ]] || die "runtime state is invalid"
    RUNTIME_DIR="$(canonical_runtime_path "$(cat "$STATE_FILE")")"
}

remove_runtime() {
    if [[ ! -e "$STATE_FILE" ]]; then
        return
    fi
    [[ -f "$STATE_FILE" && ! -L "$STATE_FILE" ]] || die "refusing unsafe runtime state file"
    [[ "$(wc -l < "$STATE_FILE" | tr -d ' ')" == 1 ]] || die "refusing invalid runtime state"
    local saved
    saved="$(canonical_runtime_path "$(cat "$STATE_FILE")" 1)"
    if [[ -d "$saved" ]]; then
        rm -rf -- "$saved"
    fi
    rm -f -- "$STATE_FILE"
}

remove_labeled_resources() {
    local kind name
    while IFS= read -r name; do
        [[ -n "$name" ]] || continue
        assert_removable_name "$name"
        assert_owned "$name" container
        docker rm -f "$name" >/dev/null
    done < <(docker ps -a --filter "label=$OWNER_LABEL" --format '{{.Names}}')

    while IFS= read -r name; do
        [[ -n "$name" ]] || continue
        assert_removable_name "$name"
        assert_owned "$name" network
        docker network rm "$name" >/dev/null
    done < <(docker network ls -q --filter "label=$OWNER_LABEL" | while read -r id; do docker network inspect --format '{{.Name}}' "$id"; done)

    while IFS= read -r name; do
        [[ -n "$name" ]] || continue
        assert_removable_name "$name"
        assert_owned "$name" volume
        docker volume rm "$name" >/dev/null
    done < <(docker volume ls -q --filter "label=$OWNER_LABEL")
}

build_one_image() {
    local image="$1"
    local default_image="$2"
    local directory="$3"
    if [[ "$image" == "$default_image" ]]; then
        docker build -t "$image" "$REPO_ROOT/images/$directory"
    else
        docker image inspect "$image" >/dev/null 2>&1 || die "overridden image is unavailable: $image"
    fi
}

build_images() {
    build_one_image "$UI_IMAGE" "$DEFAULT_UI_IMAGE" p3-sandbox-ui
    build_one_image "$TERMINAL_IMAGE" "$DEFAULT_TERMINAL_IMAGE" p3-sandbox-terminal
    build_one_image "$GENERATOR_IMAGE" "$DEFAULT_GENERATOR_IMAGE" p3-sandbox-instructions-generator
    build_one_image "$SERVER_IMAGE" "$DEFAULT_SERVER_IMAGE" p3-sandbox-instructions-server
}


create_network_and_volumes() {
    local network site creds
    network="$(resource_name net)"
    site="$(resource_name site)"
    creds="$(resource_name creds)"
    ensure_name_available network "$network"
    ensure_name_available volume "$site"
    ensure_name_available volume "$creds"
    docker network create --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" "$network" >/dev/null
    docker volume create --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" "$site" >/dev/null
    docker volume create --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" "$creds" >/dev/null
}

initialize_site_volume() {
    local name site
    name="$(resource_name site-init)"
    site="$(resource_name site)"
    docker run --rm --name "$name" --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" \
        --user 0 --entrypoint sh -v "$site:/out" "$GENERATOR_IMAGE" \
        -c 'chown 1001:1001 /out && chmod 755 /out'
}

prepare_credentials() {
    local name creds status=0
    ssh-keygen -q -t ed25519 -N '' -f "$RUNTIME_DIR/id_ed25519"
    name="$(resource_name creds-init)"
    assert_removable_name "$name"
    creds="$(resource_name creds)"
    docker create --name "$name" --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" \
        --user 0 --entrypoint sh -v "$creds:/creds" "$TERMINAL_IMAGE" \
        -c 'install -m 700 -o 1000 -g 1000 -d /creds && install -m 600 -o 1000 -g 1000 /id_ed25519 /creds/id_ed25519 && printf "%s\n" "IdentityFile ~/.ssh/id_ed25519" > /creds/config && chown 1000:1000 /creds/config && chmod 600 /creds/config' >/dev/null
    docker cp "$RUNTIME_DIR/id_ed25519" "$name:/id_ed25519" || status=$?
    if (( status == 0 )); then
        docker start -a "$name" >/dev/null || status=$?
    fi
    assert_owned "$name" container
    docker rm -f "$name" >/dev/null 2>&1 || true
    (( status == 0 )) || return "$status"
}

run_generator() {
    local name status fixture
    name="$(resource_name generator)"
    fixture="$REPO_ROOT/images/p3-sandbox-ui/tests/sandbox-ui-test/instructions"
    [[ -d "$fixture" && ! -L "$fixture" && -f "$fixture/index.md" && ! -L "$fixture/index.md" ]] || die "canonical instructions fixture is missing or unsafe: $fixture/index.md"
    docker create --name "$name" --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" \
        -v "$fixture:/src:ro" -v "$(resource_name site):/out" \
        "$GENERATOR_IMAGE" /src /out >/dev/null
    docker start "$name" >/dev/null
    status="$(docker wait "$name")"
    if [[ "$status" != 0 ]]; then
        docker logs --tail 100 "$name" >&2 || true
        die "instructions generator exited with status $status"
    fi
}

compare_authorized_key() {
    local derived derived_type derived_blob ignored
    derived="$(docker run --rm \
        --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" \
        --user 1000 --entrypoint sh -v "$(resource_name creds):/home/user/.ssh:ro" \
        -e HOME=/home/user -e USER=user -e LOGNAME=user \
        "$TERMINAL_IMAGE" -c 'ssh-keygen -y -f /home/user/.ssh/id_ed25519' 2>/dev/null)" || return 1
    read -r derived_type derived_blob ignored <<< "$derived"
    [[ -n "$derived_type" && -n "$derived_blob" ]] || return 1
    docker exec \
        -e "EXPECTED_TYPE=$derived_type" -e "EXPECTED_BLOB=$derived_blob" \
        "$(resource_name ssh)" sh -c '
            awk -v t="$EXPECTED_TYPE" -v b="$EXPECTED_BLOB" '\''
                { for (i = 1; i < NF; i++) if ($i == t && $(i + 1) == b) found = 1 }
                END { exit !found }
            '\'' /config/.ssh/authorized_keys
        ' >/dev/null 2>&1
}

probe_ssh_command() {
    local quiet="${1:-1}"
    local command=(docker run --rm
        --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID"
        --network "$(resource_name net)" --user 1000 --entrypoint ssh
        -e HOME=/home/user -e USER=user -e LOGNAME=user
        -v "$(resource_name creds):/home/user/.ssh:ro" "$TERMINAL_IMAGE"
        -o BatchMode=yes -o ConnectTimeout=2 -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null
        "tux@$(resource_name ssh)" true)
    if [[ "$quiet" == 1 ]]; then
        "${command[@]}" >/dev/null 2>&1
    else
        "${command[@]}" >/dev/null
    fi
}

wait_for_ssh_service() {
    local deadline=$((SECONDS + 60)) key_ready=0 ssh_ready=0
    while (( SECONDS < deadline )); do
        if compare_authorized_key; then
            key_ready=1
            if probe_ssh_command; then
                ssh_ready=1
                return
            fi
        fi
        sleep 1
    done
    printf 'SSH readiness failed (authorized key matched: %s; SSH command succeeded: %s)\n' "$key_ready" "$ssh_ready" >&2
    if (( key_ready == 1 )); then
        probe_ssh_command 0 || true
    fi
    docker logs --tail 100 "$(resource_name ssh)" >&2 || true
    die "SSH target did not become ready within 60 seconds"
}

check_ssh_before_publish() {
    compare_authorized_key || die "staged SSH key does not match target authorization"
}

start_stack() {
    local network public_key
    network="$(resource_name net)"
    public_key="$(cat "$RUNTIME_DIR/id_ed25519.pub")"

    docker run -d --name "$(resource_name ssh)" --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" \
        --network "$network" -e "PUBLIC_KEY=$public_key" -e USER_NAME=tux -e LISTEN_PORT=22 \
        "$SSH_IMAGE" >/dev/null
    wait_for_ssh_service
    check_ssh_before_publish

    run_generator

    docker run -d --name "$(resource_name instructions)" --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" \
        --network "$network" -p "127.0.0.1:$INSTRUCTIONS_PORT:8080" \
        -v "$(resource_name site):/site:ro" "$SERVER_IMAGE" /site >/dev/null

    docker run -d --name "$(resource_name terminal)" --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" \
        --network "$network" -p "127.0.0.1:$TERMINAL_PORT:7681" \
        -v "$(resource_name creds):/home/user/.ssh:ro" \
        -e "TERMINAL_CONFIG=$(render_terminal_config)" "$TERMINAL_IMAGE" >/dev/null

    docker run -d --name "$(resource_name ui)" --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" \
        --network "$network" -p "127.0.0.1:$UI_PORT:80" \
        -e "UI_CONFIG=$(render_ui_config)" "$UI_IMAGE" >/dev/null
}

wait_http() {
    local url="$1"
    local timeout="${2:-60}"
    local deadline=$((SECONDS + timeout))
    while (( SECONDS < deadline )); do
        curl -fsS --max-time 2 "$url" >/dev/null 2>&1 && return
        sleep 1
    done
    printf 'HTTP endpoint did not become ready: %s\n' "$url" >&2
    return 1
}

assert_container_running() {
    [[ "$(docker inspect --format '{{.State.Running}}' "$1" 2>/dev/null || true)" == true ]]
}

verify_resource_ownership() {
    local expected_run="" actual_owner actual_run kind name resource_kind
    while read -r resource_kind kind; do
        name="$(resource_name "$kind")"
        case "$resource_kind" in
            container)
                actual_owner="$(docker inspect --format "{{index .Config.Labels \"$OWNER_LABEL_KEY\"}}" "$name" 2>/dev/null)" || return 1
                actual_run="$(docker inspect --format "{{index .Config.Labels \"$RUN_LABEL_KEY\"}}" "$name" 2>/dev/null)" || return 1
                ;;
            network|volume)
                actual_owner="$(docker "$resource_kind" inspect --format "{{index .Labels \"$OWNER_LABEL_KEY\"}}" "$name" 2>/dev/null)" || return 1
                actual_run="$(docker "$resource_kind" inspect --format "{{index .Labels \"$RUN_LABEL_KEY\"}}" "$name" 2>/dev/null)" || return 1
                ;;
        esac
        [[ "$actual_owner" == "$OWNER_LABEL_VALUE" && -n "$actual_run" ]] || return 1
        if [[ -z "$expected_run" ]]; then expected_run="$actual_run"; fi
        [[ "$actual_run" == "$expected_run" ]] || return 1
    done <<EOF
network net
volume site
volume creds
container ui
container terminal
container instructions
container ssh
container generator
EOF
    RUN_ID="$expected_run"
}

verify_containers_running() {
    local kind
    for kind in ui terminal instructions ssh; do
        assert_container_running "$(resource_name "$kind")" || return 1
    done
}

verify_image_ids() {
    local kind image expected actual
    while read -r kind image; do
        expected="$(docker image inspect --format '{{.Id}}' "$image" 2>/dev/null)" || return 1
        actual="$(docker inspect --format '{{.Image}}' "$(resource_name "$kind")" 2>/dev/null)" || return 1
        [[ "$actual" == "$expected" ]] || return 1
    done <<EOF
ui $UI_IMAGE
terminal $TERMINAL_IMAGE
instructions $SERVER_IMAGE
generator $GENERATOR_IMAGE
ssh $SSH_IMAGE
EOF
}

verify_generator() {
    [[ "$(docker inspect --format '{{.State.ExitCode}}' "$(resource_name generator)" 2>/dev/null || true)" == 0 ]]
}

verify_site_owner() {
    [[ "$(docker run --rm --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=$RUN_ID" --entrypoint sh -v "$(resource_name site):/site:ro" "$GENERATOR_IMAGE" -c "stat -c '%u' /site" 2>/dev/null)" == 1001 ]]
}

verify_host_http() {
    wait_http "http://localhost:$INSTRUCTIONS_PORT/" 60 &&
        wait_http "http://localhost:$TERMINAL_PORT/" 60 &&
        wait_http "http://localhost:$UI_PORT/" 60
}

verify_terminal_markers() {
    local clipboard shortcut
    clipboard="$(docker exec "$(resource_name terminal)" sh -c 'grep -c P3_CLIPBOARD_BRIDGE_START /usr/share/ttyd/index.html' 2>/dev/null)" || return 1
    shortcut="$(docker exec "$(resource_name terminal)" sh -c 'grep -c P3_SHORTCUT_GUARD_START /usr/share/ttyd/index.html' 2>/dev/null)" || return 1
    [[ "$clipboard" == 1 && "$shortcut" == 1 ]]
}
verify_ssh() {
    docker exec "$(resource_name terminal)" sh -c 'test "$(id -u)" = 1000 && test -r /home/user/.ssh/id_ed25519 && test "$(stat -c %u /home/user/.ssh/id_ed25519)" = 1000 && test "$(stat -c %a /home/user/.ssh/id_ed25519)" = 600' >/dev/null 2>&1 || return 1
    compare_authorized_key || return 1
    docker exec "$(resource_name terminal)" ssh \
        -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null \
        "tux@$(resource_name ssh)" true >/dev/null 2>&1
}

verify_ui_config() {
    local html
    html="$(curl -fsS --max-time 5 "http://localhost:$UI_PORT/")" || return 1
    [[ "$html" == *"http://localhost:$INSTRUCTIONS_PORT"* &&
       "$html" == *"http://localhost:$TERMINAL_PORT"* &&
       "$html" == *"$EXPIRY"* &&
       "$html" != *UI_CONFIG_PLACEHOLDER* ]]
}

write_dom_probe() {
    cat > "$RUNTIME_DIR/probe.cjs" <<'JAVASCRIPT'
const puppeteer = require('puppeteer-core');

const normalize = (value) => new URL(value).href;
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

(async () => {
  const browser = await puppeteer.launch({
    executablePath: '/usr/bin/chromium-browser',
    args: ['--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage'],
  });
  try {
    const page = await browser.newPage();
    await page.goto(process.env.UI_URL, { waitUntil: 'domcontentloaded', timeout: 15000 });
    const deadline = Date.now() + 15000;
    let result;
    while (Date.now() < deadline) {
      result = await page.evaluate(() => Array.from(document.querySelectorAll('iframe')).map((frame) => ({
        src: frame.src,
        hasAllow: frame.hasAttribute('allow'),
        allow: frame.getAttribute('allow'),
      })));
      const sources = new Set(result.map((frame) => normalize(frame.src)));
      if (result.length === 2 && sources.has(normalize(process.env.INSTR_URL)) && sources.has(normalize(process.env.TERM_URL))) break;
      await sleep(100);
    }
    if (!result || result.length !== 2) throw new Error(`expected 2 parent iframe elements, found ${result ? result.length : 0}`);
    const instructions = result.find((frame) => normalize(frame.src) === normalize(process.env.INSTR_URL));
    const terminal = result.find((frame) => normalize(frame.src) === normalize(process.env.TERM_URL));
    if (!instructions || !terminal) throw new Error('expected iframe src values were not present');
    if (instructions.hasAllow && instructions.allow !== '') throw new Error('instructions iframe delegates permissions');
    const tokens = (terminal.allow || '').split(';').map((token) => token.trim()).filter(Boolean).sort();
    if (tokens.join(',') !== 'clipboard-read,clipboard-write') throw new Error('terminal clipboard delegation is incorrect');
    const delegated = result.filter((frame) => (frame.allow || '').includes('clipboard-'));
    if (delegated.length !== 1) throw new Error('clipboard delegation must appear on exactly one iframe');
  } finally {
    await browser.close();
  }
})().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
JAVASCRIPT
}

verify_dom_probe() {
    write_dom_probe
    local name status=0
    name="$(resource_name chrome-probe)"
    assert_removable_name "$name"
    docker create --name "$name" \
        --label "$OWNER_LABEL" --label "$RUN_LABEL_KEY=${RUN_ID:-verify}" \
        --network "$(resource_name net)" --entrypoint node \
        -e NODE_PATH=/usr/src/app/node_modules \
        -e "UI_URL=http://$(resource_name ui):80/" \
        -e "INSTR_URL=http://localhost:$INSTRUCTIONS_PORT" \
        -e "TERM_URL=http://localhost:$TERMINAL_PORT" \
        "$CHROME_IMAGE" /probe.cjs >/dev/null
    docker cp "$RUNTIME_DIR/probe.cjs" "$name:/probe.cjs" || status=$?
    if (( status == 0 )); then
        docker start -a "$name" || status=$?
    fi
    assert_owned "$name" container
    docker rm -f "$name" >/dev/null 2>&1 || true
    (( status == 0 ))
}

verify_port_binding() {
    local kind port output
    while read -r kind port; do
        output="$(docker port "$(resource_name "$kind")" 2>/dev/null)" || return 1
        [[ "$output" == *"127.0.0.1:$port"* ]] || return 1
        [[ "$output" != *"0.0.0.0:"* && "$output" != *"[::]:"* ]] || return 1
    done <<EOF
ui $UI_PORT
instructions $INSTRUCTIONS_PORT
terminal $TERMINAL_PORT
EOF
}

diagnose() {
    local kind name owner
    for kind in ui terminal instructions ssh generator; do
        name="$(resource_name "$kind")"
        if resource_exists container "$name"; then
            owner="$(docker inspect --format "{{index .Config.Labels \"$OWNER_LABEL_KEY\"}}" "$name" 2>/dev/null || true)"
            if [[ "$owner" == "$OWNER_LABEL_VALUE" ]]; then
                printf '\n[%s]\n' "$name" >&2
                docker logs --tail 100 "$name" >&2 || true
            fi
        fi
    done
}

run_invariant() {
    local description="$1"
    shift
    if "$@"; then
        log "OK: $description"
    else
        printf 'Verification failed: %s\n' "$description" >&2
        diagnose
        return 1
    fi
}

action_down() {
    check_command docker
    resolve_docker_endpoint >/dev/null
    docker info >/dev/null 2>&1 || die "local Docker daemon is not reachable"
    remove_labeled_resources
    remove_runtime
    log "Sandbox UI test resources removed"
}

action_up() {
    check_prereqs
    validate_settings
    remove_labeled_resources
    remove_runtime
    assert_port_free "$UI_PORT" SANDBOX_UI_TEST_UI_PORT
    assert_port_free "$INSTRUCTIONS_PORT" SANDBOX_UI_TEST_INSTRUCTIONS_PORT
    assert_port_free "$TERMINAL_PORT" SANDBOX_UI_TEST_TERMINAL_PORT

    RUN_ID="$(date +%s)-$$"
    local complete=0 previous_exit_trap previous_err_trap previous_int_trap previous_term_trap
    previous_exit_trap="$(trap -p EXIT)"
    previous_err_trap="$(trap -p ERR)"
    previous_int_trap="$(trap -p INT)"
    previous_term_trap="$(trap -p TERM)"
    cleanup_failed_up() {
        local status=$?
        trap - ERR INT TERM EXIT
        if (( complete == 0 )); then
            remove_labeled_resources || true
            if [[ -e "$STATE_FILE" ]]; then
                remove_runtime || true
            fi
            if [[ -n "${RUNTIME_DIR:-}" && -e "${RUNTIME_DIR:-}" ]]; then
                local orphan_runtime
                orphan_runtime="$(canonical_runtime_path "${RUNTIME_DIR:-}")" && rm -rf -- "$orphan_runtime" || true
            fi
        fi
        exit "$status"
    }
    trap cleanup_failed_up ERR EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM
    create_runtime
    build_images
    create_network_and_volumes
    initialize_site_volume
    prepare_credentials
    start_stack
    complete=1
    trap - ERR INT TERM EXIT
    [[ -z "$previous_exit_trap" ]] || eval "$previous_exit_trap"
    [[ -z "$previous_err_trap" ]] || eval "$previous_err_trap"
    [[ -z "$previous_int_trap" ]] || eval "$previous_int_trap"
    [[ -z "$previous_term_trap" ]] || eval "$previous_term_trap"
    log "Sandbox UI test stack started"
}

action_verify() {
    check_prereqs
    validate_settings
    load_runtime
    RUN_ID=""
    run_invariant "resource ownership and run labels agree" verify_resource_ownership
    run_invariant "owned containers are running" verify_containers_running
    run_invariant "running image IDs match current tags" verify_image_ids
    run_invariant "instructions generator completed" verify_generator
    run_invariant "site volume root is UID 1001" verify_site_owner
    run_invariant "host HTTP endpoints are ready" verify_host_http
    run_invariant "terminal browser integrations are injected once" verify_terminal_markers
    run_invariant "SSH key ownership, authorization, and command execution" verify_ssh
    run_invariant "UI configuration is injected" verify_ui_config
    run_invariant "parent iframe clipboard delegation" verify_dom_probe
    run_invariant "published ports bind only to loopback" verify_port_binding
}

action_logs() {
    check_prereqs
    local pids=()
    local kind name
    for kind in ui terminal instructions ssh; do
        name="$(resource_name "$kind")"
        assert_owned "$name" container
        docker logs -f -t "$name" 2>&1 | sed "s/^/[$name] /" &
        pids+=("$!")
    done
    wait "${pids[@]}"
}

action_run() {
    local cleaned=0
    cleanup_run() {
        if (( cleaned == 0 )); then
            cleaned=1
            action_down || true
        fi
    }
    trap cleanup_run INT TERM EXIT
    action_up
    action_verify
    local url="http://localhost:$UI_PORT/"
    log "Open: $url"
    if [[ "$OPEN_BROWSER" == 1 ]]; then
        if command -v open >/dev/null 2>&1; then
            open "$url"
        elif command -v xdg-open >/dev/null 2>&1; then
            xdg-open "$url"
        fi
    fi
    action_logs
}

internal_dispatch() {
    local helper="${1:-}"
    shift || true
    case "$helper" in
        validate_port) [[ $# == 2 ]] || die "internal validate_port requires NAME VALUE"; validate_port "$@" ;;
        resource_name) [[ $# == 1 ]] || die "internal resource_name requires KIND"; resource_name "$@" ;;
        render_ui_config) [[ $# == 0 ]] || die "internal render_ui_config takes no arguments"; render_ui_config ;;
        render_terminal_config) [[ $# == 0 ]] || die "internal render_terminal_config takes no arguments"; render_terminal_config ;;
        *) die "unknown internal helper: $helper" ;;
    esac
}

usage() {
    printf 'Usage: %s <run|up|verify|logs|down>\n' "$0" >&2
    exit 2
}

main() {
    local action="${1:-}"
    shift || true
    case "$action" in
        run) [[ $# == 0 ]] || usage; action_run ;;
        up) [[ $# == 0 ]] || usage; action_up ;;
        verify) [[ $# == 0 ]] || usage; action_verify ;;
        logs) [[ $# == 0 ]] || usage; action_logs ;;
        down) [[ $# == 0 ]] || usage; action_down ;;
        internal) internal_dispatch "$@" ;;
        *) usage ;;
    esac
}

main "$@"
