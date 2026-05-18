#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
    echo "usage: $(basename "$0") <major|minor|patch>" >&2
    exit 1
fi

level=$1

current=$(git tag --list 'v*.*.*' --sort=-v:refname | head -n1)
current=${current:-v0.0.0}

IFS=. read -r major minor patch <<<"${current#v}"

case $level in
    major) next="$((major + 1)).0.0" ;;
    minor) next="${major}.$((minor + 1)).0" ;;
    patch) next="${major}.${minor}.$((patch + 1))" ;;
    *)
        echo "error: unknown level '$level' (expected major, minor, or patch)" >&2
        exit 1
        ;;
esac

echo "v${next}"
