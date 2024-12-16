# syntax=docker/dockerfile:1.10

FROM golang as go-image
FROM pivotalcfreleng/kiln:v0.77.0 as kiln

FROM ruby:3.2.0 as builder
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts
ADD git@github.com:pivotal-cf/ops-manager.git#v2.10.66 /tmp/ops-manager

FROM ruby:3.2.0

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

# Install ops-manifest
#   assumes ops-manifest repo was cloned into ./vendor/ops-manager/gems/ops-manifest
COPY --from=builder /tmp/ops-manager /tmp/ops-manager
WORKDIR /tmp/ops-manager/gems/ops-manifest

RUN bundle install && \
  rm -rf *.gem && \
  bundle exec gem build ops-manifest.gemspec && \
  gem install ops-manifest-*.gem --no-document && \
  gem uninstall json && \
  gem install json --version 2.7.1
RUN which ops-manifest
