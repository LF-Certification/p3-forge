#!/usr/bin/env bash
# Open a login shell on the target VM, starting in REMOTE_WORKDIR.
workdir=$(printf %s "${REMOTE_WORKDIR:?}" | sed "s/'/'\\\\''/g")
exec ssh -tt -o BatchMode=yes -l "${TARGET_USER:?}" -- "${TARGET_HOST:?}" \
    "cd -- '$workdir' && exec \"\${SHELL:-/bin/sh}\" -l"
