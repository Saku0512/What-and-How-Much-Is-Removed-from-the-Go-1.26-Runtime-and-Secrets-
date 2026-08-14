#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
run_dir="${1:-${repo_dir}/artifacts/manual}"
bin_dir="${run_dir}/bin"
mkdir -p "${bin_dir}"

if ! command -v go >/dev/null 2>&1; then
  echo "go was not found; Go 1.26 or newer is required" >&2
  exit 1
fi
if ! command -v gcore >/dev/null 2>&1; then
  echo "gcore was not found; install gdb first" >&2
  exit 1
fi

export GOEXPERIMENT=runtimesecret

go build -gcflags='-m=2' -o "${bin_dir}/subject" "${repo_dir}/cmd/subject" \
  2>"${run_dir}/escape-analysis.txt"
go build -o "${bin_dir}/scanner" "${repo_dir}/cmd/scanner"

mapfile -t cases < <("${bin_dir}/subject" -list)
summary="${run_dir}/summary.tsv"
printf 'case\toccurrences\n' >"${summary}"

cleanup_pid=""
cleanup() {
  if [[ -n "${cleanup_pid}" ]] && kill -0 "${cleanup_pid}" 2>/dev/null; then
    kill "${cleanup_pid}" 2>/dev/null || true
    wait "${cleanup_pid}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

for case_name in "${cases[@]}"; do
  log_file="${run_dir}/${case_name}.log"
  core_prefix="${run_dir}/${case_name}.core"

  "${bin_dir}/subject" -mode "${case_name}" >"${log_file}" 2>&1 &
  cleanup_pid=$!

  ready=0
  for _ in $(seq 1 200); do
    if grep -qx 'READY' "${log_file}" 2>/dev/null; then
      ready=1
      break
    fi
    if ! kill -0 "${cleanup_pid}" 2>/dev/null; then
      break
    fi
    sleep 0.05
  done

  if [[ "${ready}" != 1 ]]; then
    echo "${case_name}: subject did not become ready" >&2
    sed -n '1,120p' "${log_file}" >&2
    exit 1
  fi

  if [[ "${USE_SUDO_GCORE:-0}" == 1 ]]; then
    sudo gcore -o "${core_prefix}" "${cleanup_pid}" >/dev/null
  else
    gcore -o "${core_prefix}" "${cleanup_pid}" >/dev/null
  fi
  core_file="${core_prefix}.${cleanup_pid}"

  kill "${cleanup_pid}" 2>/dev/null || true
  wait "${cleanup_pid}" 2>/dev/null || true
  cleanup_pid=""

  if [[ ! -r "${core_file}" ]]; then
    sudo chown "$(id -u):$(id -g)" "${core_file}"
  fi

  count=$("${bin_dir}/scanner" -count-only "${core_file}")
  printf '%s\t%s\n' "${case_name}" "${count}" | tee -a "${summary}"
  "${bin_dir}/scanner" "${core_file}" >"${run_dir}/${case_name}.json"

  if [[ "${KEEP_CORES:-0}" != 1 ]]; then
    rm -f "${core_file}"
  fi
done

echo
column -t -s $'\t' "${summary}" 2>/dev/null || cat "${summary}"
