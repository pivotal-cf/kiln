# syntax=docker/dockerfile:1.10

FROM golang AS go-image
FROM pivotalcfreleng/kiln:v0.103.0 AS kiln

FROM ruby:3.4
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan github.gwd.broadcom.net >> ~/.ssh/known_hosts
RUN --mount=type=ssh git clone git@github.gwd.broadcom.net:TNZ/ops-manifest.git /tmp/ops-manifest \
    && cd /tmp/ops-manifest && git submodule update --init --recursive

# Install Go
COPY --from=go-image /usr/local/go/ /usr/local/go/
ENV GOROOT=/usr/local/go/
ENV PATH="$GOROOT/bin:/root/go/bin:$PATH"

# Install Kiln
COPY --from=kiln /kiln /usr/local/bin/kiln

# Install JQ
RUN apt-get update && apt-get install jq -y

# Install Ginkgo
RUN go install github.com/onsi/ginkgo/ginkgo@latest && touch $HOME/.ack-ginkgo-rc

# Install NPM
ENV NODE_VERSION=18.16.0
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
ENV NVM_DIR=/root/.nvm
RUN . "$NVM_DIR/nvm.sh" && nvm install ${NODE_VERSION}
RUN . "$NVM_DIR/nvm.sh" && nvm use v${NODE_VERSION}
RUN . "$NVM_DIR/nvm.sh" && nvm alias default v${NODE_VERSION}
ENV PATH="/root/.nvm/versions/node/v${NODE_VERSION}/bin:${PATH}"

# Install Ops Manifest
WORKDIR /tmp/ops-manifest

RUN bundle install && \
  rm -rf *.gem && \
  bundle exec gem build ops-manifest.gemspec && \
  gem install ops-manifest-*.gem --no-document && \
  gem uninstall json && \
  gem install json --version 2.7.1 && \
  gem uninstall concurrent-ruby --force && \
  gem install concurrent-ruby --version 1.3.4 
RUN which ops-manifest
