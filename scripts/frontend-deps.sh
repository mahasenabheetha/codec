# Sourced by dev.sh and desktop.sh (bash). frontend_deps installs the
# frontend's packages when package-lock.json has changed since the last
# install. It never leaves frontend/node_modules empty or half-installed,
# and it never hangs the build:
#
#  - "changed" is by content: a git checkout or pull that rewrites the
#    lockfile without changing it doesn't count (its mtime would);
#  - npm ci deletes node_modules before it downloads, so the old one is
#    set aside first (node_modules.prev) and put back if the install
#    fails, is interrupted (Ctrl+C) or was killed in an earlier run;
#  - the install fails fast: no retries, a short timeout per download and
#    a limit on the whole install (CODEC_NPM_TIMEOUT seconds, 180 by
#    default), for a registry that can't be reached (offline, a proxy, a
#    company certificate npm doesn't trust);
#  - a lockfile whose install failed is not tried again on every build:
#    the build goes on with the packages already there and says so.
#    CODEC_NPM_RETRY=1, or `npm --prefix frontend ci` by hand, tries
#    again.

# Put the packages set aside by an install that didn't finish back.
_frontend_restore() {
  local mods=frontend/node_modules
  [[ -d $mods.prev ]] || return 0
  rm -rf "$mods"
  mv "$mods.prev" "$mods"
}

frontend_deps() {
  local dir=frontend lock=frontend/package-lock.json
  local mods=$dir/node_modules stamp=$dir/node_modules/.codec-lock
  local failed=$dir/node_modules/.codec-lock-failed
  local want limit=${CODEC_NPM_TIMEOUT:-180}
  want=$(git hash-object "$lock")

  # Left over from a run that was killed mid-install: what is in
  # node_modules now is incomplete unless its stamp says otherwise.
  if [[ -d $mods.prev ]]; then
    if [[ -f $stamp && $(<"$stamp") == "$want" ]]; then
      rm -rf "$mods.prev"
    else
      echo "note: the last install didn't finish; putting the earlier packages back." >&2
      _frontend_restore
    fi
  fi

  if [[ -d $mods && -f $stamp && $(<"$stamp") == "$want" ]]; then
    return 0
  fi
  # This lockfile's install failed before: don't wait for it again.
  if [[ -d $mods && -f $failed && $(<"$failed") == "$want" && -z ${CODEC_NPM_RETRY:-} ]]; then
    echo "note: frontend packages are older than package-lock.json (the last install failed)." >&2
    echo "      Building with them. To try again, set CODEC_NPM_RETRY=1 for this command." >&2
    return 0
  fi

  if [[ -d $mods ]]; then
    rm -rf "$mods.prev"
    mv "$mods" "$mods.prev"
  fi
  # Ctrl+C during the install: put the packages back before leaving.
  trap '_frontend_restore; trap - INT TERM; exit 130' INT TERM
  local -a run=(npm --prefix "$dir" ci --prefer-offline --no-audit --no-fund --fetch-retries=0 --fetch-timeout=20000)
  # coreutils timeout (Git Bash, Linux; not stock macOS): a limit on the
  # whole install, for a download that stalls without failing.
  if command -v timeout >/dev/null 2>&1; then
    run=(timeout -k 10 "$limit" "${run[@]}")
  fi
  local ok=0
  "${run[@]}" || ok=$?
  trap - INT TERM
  if [[ $ok -eq 0 ]]; then
    echo "$want" >"$stamp"
    rm -rf "$mods.prev"
    return 0
  fi

  if [[ -d $mods.prev ]]; then
    _frontend_restore
    echo "$want" >"$failed"
    if [[ $ok -eq 124 || $ok -eq 137 ]]; then
      echo "warning: npm ci took longer than ${limit}s and was stopped; kept the packages already installed." >&2
    else
      echo "warning: npm ci failed (see above); kept the packages already installed." >&2
    fi
    echo "         They are older than package-lock.json. When the registry can be reached" >&2
    echo "         (a company certificate: npm config set cafile <root-cert.pem>), run" >&2
    echo "         'npm --prefix frontend ci', or build again with CODEC_NPM_RETRY=1." >&2
    return 0
  fi
  echo "error: npm ci failed and there were no packages to fall back on." >&2
  return 1
}
