#!/usr/bin/env bash
# Baut ein mitgeliefertes Image und lädt es in die sbx-Runtime.
# Aufruf: images/build.sh <claude|copilot> <base|kotlin|react|fullstack|all> [--no-docker]
set -euo pipefail

agent="${1:?Agent fehlt}"
layer="${2:?Ebene fehlt}"
suffix=""
template_suffix="-docker"
if [[ "${3:-}" == "--no-docker" ]]; then
  suffix="-nodocker"
  template_suffix=""
fi

case "$agent" in
  claude) variant=claude-code ;;
  copilot) variant=copilot ;;
  *) echo "Unbekannter Agent: $agent" >&2; exit 2 ;;
esac

dir="$(cd "$(dirname "$0")" && pwd)"

build() {
  local name="$1" dockerfile="$2" base="$3"
  local tag="boxai-${name}-${agent}${suffix}:latest"
  docker build -f "$dir/$dockerfile" --build-arg "BASE=$base" --build-arg "AGENT=$agent" -t "$tag" "$dir"
}

load() {
  local tag="boxai-$1-${agent}${suffix}:latest" tar
  tar="$(mktemp -t boxai-image.XXXXXX)"
  trap 'rm -f "$tar"' RETURN
  docker image save "$tag" -o "$tar"
  sbx template load "$tar"
}

base_ref="boxai-base-${agent}${suffix}:latest"
kotlin_ref="boxai-kotlin-${agent}${suffix}:latest"

need() {
  docker image inspect "$1" >/dev/null 2>&1 || "${@:2}"
}

build_base() { build base base.Dockerfile "docker/sandbox-templates:${variant}${template_suffix}"; }
build_kotlin() { need "$base_ref" build_base; build kotlin kotlin.Dockerfile "$base_ref"; }
build_react() { need "$base_ref" build_base; build react react.Dockerfile "$base_ref"; }
build_fullstack() { need "$kotlin_ref" build_kotlin; build fullstack react.Dockerfile "$kotlin_ref"; }

case "$layer" in
  base|kotlin|react|fullstack)
    "build_$layer"
    load "$layer"
    ;;
  all)
    build_base; build_kotlin; build_react; build_fullstack
    for name in base kotlin react fullstack; do load "$name"; done
    ;;
  *) echo "Unbekannte Ebene: $layer" >&2; exit 2 ;;
esac
