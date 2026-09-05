#!/usr/bin/env bash
set -euo pipefail

readonly project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly workspace_root="$(dirname "$project_root")"

checkout_dependency() {
  local name="$1"
  local repository="$2"
  local commit="$3"
  local destination="$workspace_root/$name"

  if [[ -e "$destination" ]]; then
    printf '测试依赖目录已存在: %s\n' "$destination" >&2
    return 1
  fi

  git init --quiet "$destination"
  git -C "$destination" remote add origin "https://github.com/goark-projects/$repository.git"
  git -C "$destination" fetch --quiet --depth=1 origin "$commit"
  git -C "$destination" checkout --quiet --detach FETCH_HEAD
}

export GIT_TERMINAL_PROMPT=0

checkout_dependency goark goark d90ef90bc395903190912a163af898267af521af
checkout_dependency goark-boot goark-boot f0bf0d486a4d7cffd769836241aa2113a5bfbecb
checkout_dependency goark-boot-contrib-log goark-boot-contrib-log 5972a947a031b54d1ee96b2c18bac336ffc14a38
checkout_dependency arkarta arkarta 45ebc42e54af2b43cf620bbc6831e738fdddca59
checkout_dependency arkhos arkhos ffbc091be55c70daae67bc41a78b11bc2dcfb923
checkout_dependency goark-boot-contrib-arkhos goark-boot-contrib-arkhos 6e4b2d04988706cd86b7b5b5e4f7d61a73002125
checkout_dependency goark-boot-contrib-web goark-boot-contrib-web 1aa8bcdd2befb83cba9905301d2c0216ea1deaf6
