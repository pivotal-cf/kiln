# Base images: host must match DockerVirtualRegistryHost in internal/test/container.go (AuthConfigs).
FROM tas-rel-eng-docker-virtual.usw1.packages.broadcom.com/golang AS go-image
FROM tas-rel-eng-docker-virtual.usw1.packages.broadcom.com/pivotalcfreleng/kiln:v0.110.0-rc2 AS kiln

FROM tas-rel-eng-docker-virtual.usw1.packages.broadcom.com/ruby:3.4.8 AS builder
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts

FROM tas-rel-eng-docker-virtual.usw1.packages.broadcom.com/ruby:3.4.8

# Install Go
COPY --from=go-image /usr/local/go/ /usr/local/go/
ENV GOROOT=/usr/local/go/
ENV PATH="$GOROOT/bin:/root/go/bin:$PATH"

# Install Kiln
COPY --from=kiln /kiln /usr/local/bin/kiln

# Install JQ
RUN apt-get update && apt-get install jq -y

ARG ARTIFACTORY_USERNAME
ARG ARTIFACTORY_PASSWORD
ENV ARTIFACTORY_USERNAME=${ARTIFACTORY_USERNAME}
ENV ARTIFACTORY_PASSWORD=${ARTIFACTORY_PASSWORD}

# Install Ginkgo (module download via Artifactory Go virtual; credentials from build args).
ARG GOTOOLCHAIN=local
ENV GOTOOLCHAIN=${GOTOOLCHAIN}
ENV GOPROXY=https://${ARTIFACTORY_USERNAME}:${ARTIFACTORY_PASSWORD}@usw1.packages.broadcom.com/artifactory/api/go/tas-rel-eng-go-virtual
ENV GOSUMDB=off
# Pure-Go install; avoid invoking gcc. On linux/arm64 (e.g. Podman on Apple Silicon),
# cgo can fail with: gcc: error: unrecognized command-line option '-m64'.
ENV CGO_ENABLED=0
RUN go install github.com/onsi/ginkgo/ginkgo@latest

# Install NodeJS
RUN apt-get update && apt-get install nodejs npm -y

RUN gem source -a https://${ARTIFACTORY_USERNAME}:${ARTIFACTORY_PASSWORD}@usw1.packages.broadcom.com/artifactory/api/gems/tas-rel-eng-gem-dev-local/
RUN gem install --verbose ops-manifest -v 0.0.4.pre

RUN which ops-manifest
