FROM ubuntu:22.04

RUN apt-get update && apt-get install -y locales && rm -rf /var/lib/apt/lists/* \
	&& localedef -i en_US -c -f UTF-8 -A /usr/share/locale/locale.alias en_US.UTF-8
ENV LANG en_US.utf8

RUN apt-get update && apt install -y curl xz-utils

RUN mkdir -p /app

WORKDIR /app

RUN /bin/bash -c "sh <(curl -L https://nixos.org/nix/install) --daemon --yes"

ADD flake.nix flake.lock /app/

RUN echo "experimental-features = nix-command flakes" >> /etc/nix/nix.conf

CMD ["/bin/bash"]
