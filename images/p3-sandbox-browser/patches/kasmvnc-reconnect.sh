#!/bin/sh
# Make the bundled KasmVNC client survive a dropped WebSocket.
#
# Upstream (linuxserver/firefox, KasmVNC www bundle):
#   - disconnectFinished() drops UI.rfb but leaves the 5s keep-alive interval
#     running, so every tick throws "Cannot read properties of undefined
#     (reading 'lastActiveAt')" into the error overlay.
#   - The reconnect setting is honoured only for clean closes. A proxy or
#     network cut arrives as an unclean close (code 1006), so a session that
#     was working simply dies on screen.
#
# This script edits the shipped bundle in place at image build time and fails
# the build if the anchors it relies on are not found exactly once, so a base
# image bump that changes the client cannot silently ship without it.
set -eu

bundle=/usr/share/kasmvnc/www/dist/main.bundle.js
kclient=/kclient/public/index.html

expect_once() {
  n=$(grep -c -F -- "$2" "$1")
  [ "$n" -eq 1 ] || { echo "patch anchor '$2' found $n times in $1" >&2; exit 1; }
}

expect_once "$bundle" "    UI.rfb = undefined;"
expect_once "$kclient" "show_control_bar=true"

# Replace the single anchor line with the guarded teardown + reconnect.
sed -i 's|^    UI\.rfb = undefined;$|    UI.rfb = undefined;\
    clearInterval(UI._sessionTimeoutInterval);\
    if (wasConnected \&\& UI.getSetting("reconnect", false) === true \&\& !UI.inhibitReconnect) {\
      console.warn("kasmvnc: connection lost (clean=" + e.detail.clean + "), reconnecting");\
      UI.updateVisualState("reconnecting");\
      UI.reconnectCallback = setTimeout(UI.reconnect, parseInt(UI.getSetting("reconnect_delay")));\
      return;\
    }|' "$bundle"

sed -i 's|show_control_bar=true|show_control_bar=true\&reconnect=1\&reconnect_delay=2000|' "$kclient"

expect_once "$bundle" 'kasmvnc: connection lost'
expect_once "$kclient" 'reconnect=1&reconnect_delay=2000'
