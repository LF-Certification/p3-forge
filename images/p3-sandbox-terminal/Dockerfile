FROM node:22-alpine AS index-builder
COPY src/index.html /work/index.html
COPY src/shortcut-guard.js src/clipboard-bridge.js /work/scripts/
COPY scripts/inject-custom-scripts.mjs /work/inject-custom-scripts.mjs
RUN node /work/inject-custom-scripts.mjs \
        /work/index.html \
        /work/scripts/shortcut-guard.js \
        /work/scripts/clipboard-bridge.js

FROM tsl0922/ttyd:1.7.7

# Install necessary packages
USER root
RUN apt-get update -y && \
    apt-get install -y \
    tmux \
    openssh-client \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Create the runtime account required by OpenSSH's getpwuid lookup.
RUN groupadd --gid 1000 user && \
    useradd --uid 1000 --gid 1000 --home-dir /home/user --create-home --shell /bin/bash user && \
    install -d -m 700 -o user -g user /home/user/.ssh

# Configure tmux for the user:
# - set-clipboard + Ms override: emit OSC 52 for mouse-drag copies so the
#   browser clipboard bridge (src/clipboard-bridge.js) receives them.
# - unbind MouseDown3*: reserve right-click for browser paste. This also
#   deliberately prevents mouse-aware terminal applications from receiving
#   right-button events. The shipped tmux (3.0a) predates 'unbind -q'.
RUN printf '%s\n' \
    'set -g status off' \
    'set -g mouse on' \
    'set -g history-limit 10000' \
    'set -s set-clipboard external' \
    "set -as terminal-overrides ',*:Ms=\\E]52;%p1%s;%p2%s\\007'" \
    'unbind -n MouseDown3Pane' \
    'unbind -n M-MouseDown3Pane' \
    'unbind -n MouseDown3Status' \
    'unbind -n MouseDown3StatusLeft' \
    'unbind -n MouseDown3StatusRight' \
    > /home/user/.tmux.conf && \
    chown 1000:1000 /home/user/.tmux.conf && \
    # Validate pinned tmux defaults on an isolated server, then clean up
    # explicitly even when an assertion fails.
    TMUX_TMPDIR="$(mktemp -d)" && export TMUX_TMPDIR && \
    { validation_status=0; { \
        [ "$(tmux -V)" = 'tmux 3.0a' ] && \
        tmux -f /home/user/.tmux.conf -L p3build new-session -d 'sleep 60' && \
        tmux -L p3build show-options -sv set-clipboard | grep -qxF external && \
        tmux -L p3build show-options -gv mouse | grep -qxF on && \
        tmux -L p3build show-options -gv history-limit | grep -qxF 10000 && \
        tmux -L p3build show-options -gsv terminal-overrides | grep -qxF '*:Ms=\E]52;%p1%s;%p2%s\007' && \
        root_keys="$(tmux -L p3build list-keys -T root)" && \
        copy_keys="$(tmux -L p3build list-keys -T copy-mode)" && \
        vi_keys="$(tmux -L p3build list-keys -T copy-mode-vi)" && \
        printf '%s\n' "$root_keys" | grep -Eq '^(bind|bind-key) +-T root +WheelUpPane +' && \
        printf '%s\n' "$copy_keys" | grep -Eq '^(bind|bind-key) +-T copy-mode +WheelUpPane +' && \
        printf '%s\n' "$copy_keys" | grep -Eq '^(bind|bind-key) +-T copy-mode +WheelDownPane +' && \
        printf '%s\n' "$vi_keys" | grep -Eq '^(bind|bind-key) +-T copy-mode-vi +WheelUpPane +' && \
        printf '%s\n' "$vi_keys" | grep -Eq '^(bind|bind-key) +-T copy-mode-vi +WheelDownPane +' && \
        ! printf '%s\n' "$root_keys" | grep -Eq '^(bind|bind-key) +-T root +(M-)?MouseDown3(Pane|Status|StatusLeft|StatusRight) '; \
      } || validation_status=$?; \
      tmux -L p3build kill-server >/dev/null 2>&1 || :; \
      rm -rf "$TMUX_TMPDIR"; \
      exit "$validation_status"; \
    }

# Install browser integrations generated after ttyd's bundled application.
COPY --from=index-builder /work/index.html /usr/share/ttyd/index.html
# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Switch to the named runtime account.
USER user
ENV HOME=/home/user
ENV SHELL=/bin/bash

# Set working directory
WORKDIR /home/user

ENTRYPOINT ["/entrypoint.sh"]
