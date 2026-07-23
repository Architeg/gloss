#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "Usage: scripts/build-release.sh vMAJOR.MINOR.PATCH OUTPUT_DIRECTORY" >&2
  exit 2
fi

version="$1"
output_dir="$2"
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
repository_root="$(cd -- "$script_dir/.." && pwd -P)"

exec go run "$repository_root/scripts/releasebuild" \
  -version "$version" \
  -output "$output_dir" \
  -root "$repository_root"
