FROM tsl0922/ttyd:1.7.7

RUN apt-get update -y
RUN apt-get install -y tmux openssh-client jq

RUN echo "set -g status off" > /root/.tmux.conf

COPY entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
