# Sourced by dev.sh and desktop.sh (bash). frontend_deps installs the
# frontend's packages when package-lock.json has changed since the last
# install, and never leaves frontend/node_modules empty:
#
#  - "changed" is by content: a git checkout or pull that rewrites the
#    lockfile without changing it doesn't count (its mtime would);
#  - npm ci deletes node_modules before it downloads, so the old one is
#    set aside first and put back if the install fails (a registry that
#    can't be reached, a proxy), with a warning; the build goes on with
#    the packages already there.
frontend_deps() {
  local dir=frontend lock=frontend/package-lock.json
  local mods=$dir/node_modules stamp=$dir/node_modules/.codec-lock
  local want
  want=$(git hash-object "$lock")
  if [[ -d $mods && -f $stamp && $(<"$stamp") == "$want" ]]; then
    return 0
  fi
  if [[ -d $mods ]]; then
    rm -rf "$mods.prev"
    mv "$mods" "$mods.prev"
  fi
  if npm --prefix "$dir" ci --prefer-offline --no-audit --no-fund; then
    rm -rf "$mods.prev"
    echo "$want" >"$stamp"
    return 0
  fi
  if [[ -d $mods.prev ]]; then
    rm -rf "$mods"
    mv "$mods.prev" "$mods"
    echo "warning: npm ci failed (see above); kept the packages already installed." >&2
    echo "         Run 'npm --prefix frontend ci' again when the registry can be reached." >&2
    return 0
  fi
  echo "error: npm ci failed and there were no packages to fall back on." >&2
  return 1
}
