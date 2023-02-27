# syntax=docker/dockerfile-upstream:master-labs

FROM golang:1 as go-image
FROM pivotalcfreleng/kiln:v0.77.0 as kiln

FROM ruby:3.2.0 as builder
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts
ADD git@github.com:pivotal-cf/ops-manager.git#releases/2.10 /tmp/ops-manager

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

# Install ops-manifest
#   assumes ops-manifest repo was cloned into ./vendor/ops-manager/gems/ops-manifest
COPY --from=builder /tmp/ops-manager /tmp/ops-manager
WORKDIR /tmp/ops-manager/gems/ops-manifest

RUN bundle install && \
  rm -rf *.gem && \
  bundle exec gem build ops-manifest.gemspec && \
  gem install ops-manifest-*.gem --no-document
RUN which ops-manifest
