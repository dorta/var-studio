#!/bin/sh
set -eu

repository="${VAR_STUDIO_REPOSITORY:-dorta/var-studio}"
channel="${VAR_STUDIO_VERSION:-main}"
image="${VAR_STUDIO_IMAGE:-var-studio:local}"
default_raw="https://raw.githubusercontent.com/${repository}/${channel}"
raw_base="${VAR_STUDIO_RAW_BASE:-${default_raw}}"
install_dir="${VAR_STUDIO_INSTALL_DIR:-/opt/var-studio}"
config_file="/etc/default/var-studio"
log_dir="${VAR_STUDIO_LOG_DIR:-/var/log/var-studio}"
log_file="$log_dir/install.log"
step_total=8
step_number=0
progress_drawn=false

setup_style() {
  interactive=false
  bold=''
  dim=''
  green=''
  red=''
  reset=''
  if [ -t 1 ] && [ "${NO_COLOR:-}" = "" ]; then
    interactive=true
    bold='\033[1m'
    dim='\033[2m'
    green='\033[32m'
    red='\033[31m'
    reset='\033[0m'
  fi
}

fail() {
  printf '\n%bInstallation failed%b\n' "$red" "$reset" >&2
  printf '  %s\n' "$*" >&2
  if [ -s "${log_file:-}" ]; then
    printf '\n  Last log entries:\n' >&2
    tail -n 12 "$log_file" | sed 's/^/    /' >&2
    printf '\n  Full log: %s\n' "$log_file" >&2
  fi
  exit 1
}
download() {
  curl --retry 5 --retry-delay 2 --connect-timeout 15 "$@"
}

render_progress() {
  progress_state="$1"
  elapsed=$(($(date +%s) - started))
  minutes=$((elapsed / 60))
  seconds=$((elapsed % 60))
  bar_width=24
  completed=$((step_number - 1))
  if [ "$progress_state" = done ]; then
    completed="$step_number"
  fi
  filled=$((completed * bar_width / step_total))
  printf '\r\033[2K  ['
  position=1
  while [ "$position" -le "$bar_width" ]; do
    if [ "$position" -le "$filled" ]; then
      printf '%b#%b' "$green" "$reset"
    elif [ "$position" -eq $((filled + 1)) ] && \
      [ "$progress_state" = active ]; then
      printf '%b>%b' "$bold" "$reset"
    elif [ "$position" -eq $((filled + 1)) ] && \
      [ "$progress_state" = failed ]; then
      printf '%b!%b' "$red" "$reset"
    else
      printf '%b.%b' "$dim" "$reset"
    fi
    position=$((position + 1))
  done
  printf '] %02d/%02d  %-34s %02d:%02d' \
    "$step_number" "$step_total" "$label" "$minutes" "$seconds"
  progress_drawn=true
}

clear_progress() {
  if [ "$interactive" = true ] && [ "$progress_drawn" = true ]; then
    printf '\n'
    progress_drawn=false
  fi
}

run_step() {
  label="$1"
  shift
  step_number=$((step_number + 1))
  started="$(date +%s)"
  if [ "$interactive" = false ]; then
    printf '  [%s/%s] %-34s' "$step_number" "$step_total" "$label"
  fi
  "$@" >>"$log_file" 2>&1 &
  task_pid=$!
  while [ "$interactive" = true ] && \
    kill -0 "$task_pid" 2>/dev/null; do
    render_progress active
    sleep 1
  done
  if wait "$task_pid"; then
    if [ "$interactive" = true ]; then
      render_progress done
    else
      elapsed=$(($(date +%s) - started))
      printf ' %bDONE%b  %ss\n' "$green" "$reset" "$elapsed"
    fi
  else
    result=$?
    if [ "$interactive" = true ]; then
      render_progress failed
      printf '\n'
    else
      printf ' %bFAILED%b\n' "$red" "$reset"
    fi
    fail "$label returned exit code $result"
  fi
}

recover_system_time() {
  clock_url="https://github.com"
  if download -fsSI "$clock_url" >/dev/null 2>&1; then
    return 0
  fi
  header="$(download -kfsSIL "$clock_url" | tr -d '\r' | \
    sed -n 's/^[Dd]ate: //p' | head -n 1)"
  set -- $header
  case "${3:-}" in
    Jan) month=01 ;; Feb) month=02 ;; Mar) month=03 ;;
    Apr) month=04 ;; May) month=05 ;; Jun) month=06 ;;
    Jul) month=07 ;; Aug) month=08 ;; Sep) month=09 ;;
    Oct) month=10 ;; Nov) month=11 ;; Dec) month=12 ;;
    *) return 1 ;;
  esac
  date -u -s "$4-$month-$2 $5" >/dev/null
}

check_platform() {
  [ "$(id -u)" -eq 0 ] || return 10
  command -v curl >/dev/null 2>&1 || return 11
  command -v docker >/dev/null 2>&1 || return 12
  command -v systemctl >/dev/null 2>&1 || return 13
  if ! command -v sha256sum >/dev/null 2>&1 && \
    ! command -v openssl >/dev/null 2>&1; then
    return 14
  fi
  case "$(uname -m)" in
    aarch64 | arm64 | armv7l) return 0 ;;
    *) return 15 ;;
  esac
}

fetch_deployment() {
  for file in VERSION LICENSE NOTICE Dockerfile.runtime \
    scripts/var-studio \
    scripts/var-studio-stack \
    packaging/var-studio-stack.service \
    packaging/var-studio-demo-runner.service \
    packaging/var-studio-camera-runner.service; do
    destination="$temporary/$(basename "$file")"
    download -fsSL "$raw_base/$file" -o "$destination" || return 1
  done
}

prepare_runtime() {
  systemctl enable --now docker.service
  systemctl enable --now systemd-timesyncd.service || true
}

file_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    openssl dgst -sha256 "$1" | awk '{print $NF}'
  fi
}

fetch_application() {
  binary_name="var-studio-linux-$asset_arch"
  release_url="https://github.com/${repository}/releases/download"
  release_url="$release_url/v$release"
  mkdir -p "$image_context/data"
  download -fsSL "$release_url/$binary_name" \
    -o "$image_context/var-studio" || return 1
  download -fsSL "$release_url/SHA256SUMS" \
    -o "$temporary/SHA256SUMS" || return 1
  expected="$(awk -v name="$binary_name" '
    $2 == name { print $1; exit }
  ' "$temporary/SHA256SUMS")"
  [ -n "$expected" ] || return 1
  actual="$(file_sha256 "$image_context/var-studio")" || return 1
  [ "$actual" = "$expected" ] || return 1
  chmod 0755 "$image_context/var-studio"
  cp "$temporary/Dockerfile.runtime" "$image_context/Dockerfile"
  cp "$temporary/LICENSE" "$image_context/LICENSE"
  cp "$temporary/NOTICE" "$image_context/NOTICE"
}

wait_for_thermal_headroom() {
  waited=0
  while [ "$waited" -lt 180 ]; do
    thermal_blocked=false
    for zone in /sys/class/thermal/thermal_zone*; do
      [ -r "$zone/temp" ] || continue
      current_temperature="$(cat "$zone/temp")"
      passive_limit=''
      for trip_type in "$zone"/trip_point_*_type; do
        [ -r "$trip_type" ] || continue
        [ "$(cat "$trip_type")" = passive ] || continue
        trip_temperature="${trip_type%_type}_temp"
        [ -r "$trip_temperature" ] || continue
        passive_limit="$(cat "$trip_temperature")"
        break
      done
      if [ -n "$passive_limit" ] && \
        [ "$current_temperature" -ge "$passive_limit" ]; then
        thermal_blocked=true
        printf 'Waiting for %s to cool below %s millidegrees C\n' \
          "$zone" "$passive_limit"
        break
      fi
    done
    [ "$thermal_blocked" = false ] && return 0
    sleep 5
    waited=$((waited + 5))
  done
  return 1
}

build_image() {
  wait_for_thermal_headroom || return 1
  if docker buildx version >/dev/null 2>&1; then
    printf 'Builder: Docker Buildx\n'
    docker buildx build --load --pull --network none \
      -t "$image" "$image_context"
  else
    printf 'Builder: Docker legacy compatibility mode\n'
    docker build --pull --network none \
      -t "$image" "$image_context"
  fi
}

migrate_legacy_installation() {
  legacy_services="var-scope-stack.service"
  legacy_services="$legacy_services var-scope-demo-runner.service"
  legacy_services="$legacy_services var-scope-camera-runner.service"
  systemctl disable --now $legacy_services >/dev/null 2>&1 || true
  for legacy_container in var-scope var-scope-klog \
    var-scope-gpu var-scope-npu; do
    docker rm -f "$legacy_container" >/dev/null 2>&1 || true
  done
  for legacy_volume in var-scope-data opt_var-scope-data \
    var-scope_var-scope-data; do
    if docker volume inspect "$legacy_volume" >/dev/null 2>&1; then
      docker volume create \
        --label com.variscite.var-studio=true \
        var-studio-data >/dev/null
      legacy_mount="$(docker volume inspect \
        --format '{{.Mountpoint}}' "$legacy_volume")"
      studio_mount="$(docker volume inspect \
        --format '{{.Mountpoint}}' var-studio-data)"
      cp -a "$legacy_mount/." "$studio_mount/"
      docker volume rm "$legacy_volume" >/dev/null
    fi
  done
  docker image rm var-scope:local >/dev/null 2>&1 || true
  rm -f /etc/systemd/system/var-scope-stack.service
  rm -f /etc/systemd/system/var-scope-demo-runner.service
  rm -f /etc/systemd/system/var-scope-camera-runner.service
  rm -f /etc/default/var-scope
  rm -f /usr/bin/var-scope /usr/local/bin/var-scope
  rm -f /usr/local/sbin/var-scope
  rm -rf /opt/var-scope /var/log/var-scope
  rm -rf /run/var-scope-demo /run/var-scope-camera
  systemctl daemon-reload
}

write_config() {
  {
    printf 'VAR_STUDIO_IMAGE=%s\n' "$image"
    printf 'VAR_STUDIO_VERSION=%s\n' "$release"
    printf 'VAR_STUDIO_REPOSITORY=%s\n' "$repository"
    printf 'VAR_STUDIO_CHANNEL=%s\n' "$channel"
    printf 'VAR_STUDIO_RAW_BASE=%s\n' "$raw_base"
    printf 'VAR_STUDIO_PORT=9090\n'
  } >"$config_file"
}

install_files() {
  migrate_legacy_installation
  mkdir -p "$install_dir/bin" /usr/bin
  cp "$temporary/LICENSE" "$install_dir/LICENSE"
  cp "$temporary/NOTICE" "$install_dir/NOTICE"
  cp "$temporary/var-studio" /usr/bin/var-studio
  cp "$temporary/var-studio-stack" \
    "$install_dir/bin/var-studio-stack"
  cp "$temporary/var-studio-stack.service" \
    /etc/systemd/system/var-studio-stack.service
  cp "$temporary/var-studio-demo-runner.service" \
    /etc/systemd/system/var-studio-demo-runner.service
  cp "$temporary/var-studio-camera-runner.service" \
    /etc/systemd/system/var-studio-camera-runner.service
  chmod 0755 /usr/bin/var-studio \
    "$install_dir/bin/var-studio-stack"
  chmod 0644 "$install_dir/LICENSE" "$install_dir/NOTICE" \
    /etc/systemd/system/var-studio-stack.service \
    /etc/systemd/system/var-studio-demo-runner.service \
    /etc/systemd/system/var-studio-camera-runner.service
  printf '%s\n' "$release" >"$install_dir/VERSION"
  write_config
  docker create --network none --name "$container" "$image" >/dev/null
  docker cp "$container:/var-studio" \
    "$install_dir/bin/var-studio-server"
  chmod 0755 "$install_dir/bin/var-studio-server"
  docker rm "$container" >/dev/null
}

enable_services() {
  systemctl daemon-reload
  systemctl enable var-studio-demo-runner.service \
    var-studio-camera-runner.service var-studio-stack.service
  systemctl restart var-studio-demo-runner.service \
    var-studio-camera-runner.service
  systemctl restart var-studio-stack.service
}

wait_for_dashboard() {
  attempt=0
  while [ "$attempt" -lt 30 ]; do
    if curl -fsS http://127.0.0.1:9090/api/v1/snapshot \
      >/dev/null 2>&1; then
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  return 1
}

setup_style
printf '\n%bVAR-Studio%b\n' "$bold" "$reset"
printf 'Board Diagnostics Setup\n'
printf '%b(c) 2026 Variscite Ltd.%b\n\n' "$dim" "$reset"
printf '  %-10s %s (%s)\n' 'Device' "$(hostname)" "$(uname -m)"
printf '  %-10s %s\n\n' 'Method' \
  'Checksum-verified binary, local Docker image'

[ "$(id -u)" -eq 0 ] || fail 'run this installer as root'

mkdir -p "$log_dir"
: >"$log_file"
printf 'Started: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  >>"$log_file"

recover_system_time >>"$log_file" 2>&1 || fail \
  'unable to establish a valid system clock'

check_platform || fail \
  'requires root, Docker, curl, systemd, SHA-256, and a supported ARM CPU'
case "$(uname -m)" in
  aarch64 | arm64) asset_arch=arm64 ;;
  armv7l) asset_arch=armv7 ;;
esac

temporary="$(mktemp -d)"
container="var-studio-installer-$$"
image_context="$temporary/image"
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$temporary"
}
trap cleanup EXIT INT TERM

run_step 'Checking board and runtime' check_platform
run_step 'Downloading deployment files' fetch_deployment
release="$(tr -d '[:space:]' <"$temporary/VERSION")"
[ -n "$release" ] || fail 'downloaded VERSION is empty'
run_step 'Preparing Docker runtime' prepare_runtime
run_step 'Downloading verified application' fetch_application
run_step 'Assembling container image' build_image
run_step 'Installing system files' install_files
run_step 'Starting system services' enable_services
run_step 'Verifying dashboard' wait_for_dashboard
clear_progress

dashboard_url="$(/usr/bin/var-studio url | awk '
  $1 == "Network" { print $2; exit }
')"
printf '\n%bVAR-Studio %s installed%b\n\n' \
  "$green" "$release" "$reset"
printf '  %-12s %b%s%b\n' 'Status' "$green" 'RUNNING' "$reset"
printf '  %-12s %s\n' 'Dashboard' "$dashboard_url"
printf '  %-12s %s\n' 'Command' 'var-studio status'
printf '\n'
trap - EXIT INT TERM
cleanup
