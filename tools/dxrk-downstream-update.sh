#!/usr/bin/env bash
# dxrk-downstream-update.sh
# Pull latest changes from upstream repos and merge into Dxrk/DxrkMemory.
# Usage: ./tools/dxrk-downstream-update.sh [--dry-run] [--remote <name>]

set -euo pipefail

DRY_RUN=false
TARGET_REMOTE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    --remote) TARGET_REMOTE="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

# Remotes and their integration prefixes
declare -A REMOTES=(
  [upstream]=""                    # dxrk: already the root, no prefix
  [mempalace]=""                   # already in DxrkMemory/
  [openclaw]="integrations/openclaw"
  [hermes]="integrations/hermes"
  [dxrk-memory]="integrations/dxrk-memory"
  [dxrk-guardian]="integrations/dxrk-guardian"
  [dxrk-pi]="integrations/dxrk-pi"
  [dxrk-skills]="skills"
)

# Main branches for each remote
declare -A BRANCHES=(
  [upstream]="main"
  [mempalace]="main"
  [openclaw]="main"
  [hermes]="main"
  [dxrk-memory]="main"
  [dxrk-guardian]="main"
  [dxrk-pi]="main"
  [dxrk-skills]="main"
)

update_remote() {
  local remote="$1"
  local prefix="${REMOTES[$remote]}"
  local branch="${BRANCHES[$remote]}"

  echo "=== Fetching $remote ==="
  git fetch "$remote" --tags

  if $DRY_RUN; then
    echo "[DRY RUN] Would merge $remote/$branch into $prefix"
    local diff_count
    diff_count=$(git rev-list --count "$remote/$branch" "^$(git merge-base HEAD "$remote/$branch")" 2>/dev/null || echo "0")
    echo "  New commits: $diff_count"
    return
  fi

  if [[ -z "$prefix" ]]; then
    echo "  Skipping $remote (root project, manual merge required)"
    return
  fi

  echo "  Merging $remote/$branch -> $prefix/"

  # Use read-tree to merge with prefix
  # First, remove old content
  if [[ -d "$prefix" ]]; then
    git rm -rf "$prefix" 2>/dev/null || true
  fi

  # Merge new content with prefix
  git read-tree --prefix="$prefix/" "$remote/$branch"
  git checkout -- "$prefix/"

  # Clean up meta files that shouldn't be in the monorepo
  rm -rf "$prefix/.github/" "$prefix/.agents/" "$prefix/AGENTS.md" "$prefix/CLAUDE.md" \
         "$prefix/CONTRIBUTING.md" "$prefix/LICENSE" "$prefix/SECURITY.md" "$prefix/README.md" \
         "$prefix/.npmrc" "$prefix/.dockerignore" "$prefix/.gitattributes" "$prefix/.gitignore" \
         2>/dev/null || true

  git add "$prefix/"
  echo "  Done: $remote -> $prefix/"
}

echo "Dxrk Downstream Update Script"
echo "=============================="
echo ""

if [[ -n "$TARGET_REMOTE" ]]; then
  update_remote "$TARGET_REMOTE"
else
  for remote in "${!REMOTES[@]}"; do
    update_remote "$remote"
  done
fi

if $DRY_RUN; then
  echo ""
  echo "Dry run complete. Run without --dry-run to apply changes."
else
  echo ""
  echo "All remotes updated. Commit with:"
  echo "  git commit -m 'chore: downstream update from upstream repos'"
fi
