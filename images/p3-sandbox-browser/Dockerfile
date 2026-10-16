FROM linuxserver/firefox:139.0.4

# Patch the bundled KasmVNC client to reconnect after a dropped WebSocket
# instead of showing an error overlay. See patches/kasmvnc-reconnect.sh.
COPY patches/kasmvnc-reconnect.sh /tmp/kasmvnc-reconnect.sh
RUN sh /tmp/kasmvnc-reconnect.sh && rm /tmp/kasmvnc-reconnect.sh
