# Base images: host must match DockerVirtualRegistryHost in internal/test/container.go (AuthConfigs).
FROM tas-rel-eng-docker-virtual.usw1.packages.broadcom.com/golang AS go-image
FROM tas-rel-eng-docker-virtual.usw1.packages.broadcom.com/pivotalcfreleng/kiln:v0.110.0-rc2 AS kiln

FROM tas-rel-eng-docker-virtual.usw1.packages.broadcom.com/ruby:4.0.3 AS builder
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts

FROM tas-rel-eng-docker-virtual.usw1.packages.broadcom.com/ruby:4.0.3

# ── Stable tools — no credentials; these layers cache across credential rotation ──

# Go runtime
COPY --from=go-image /usr/local/go/ /usr/local/go/
ENV GOROOT=/usr/local/go/
ENV PATH="$GOROOT/bin:/root/go/bin:$PATH"

# Kiln binary (used by ops-manifest during manifest tests)
COPY --from=kiln /kiln /usr/local/bin/kiln

# System packages (consolidated to one layer; before credentials so they stay cached)
RUN apt-get update \
 && apt-get install --no-install-recommends -y jq nodejs npm \
 && rm -rf /var/lib/apt/lists/*

# Go toolchain settings — must appear before any `go install`
# CGO_ENABLED=0: pure-Go build avoids gcc issues on arm64 (e.g. Podman on Apple Silicon).
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=0

# ── Ginkgo — credentials scoped to this RUN only so layers above stay cached ──
# Pinned to v1.16.5 for reproducibility (last stable v1; tiles using ginkgo v2 use their own binary).

ARG ARTIFACTORY_USERNAME
ARG ARTIFACTORY_PASSWORD

RUN GOPROXY=https://${ARTIFACTORY_USERNAME}:${ARTIFACTORY_PASSWORD}@usw1.packages.broadcom.com/artifactory/api/go/tas-rel-eng-go-virtual \
    GOSUMDB=off \
    go install github.com/onsi/ginkgo/ginkgo@v1.16.5

# ── ops-manifest gem — credentials exported for gem source registration at build time ──

ENV ARTIFACTORY_USERNAME=${ARTIFACTORY_USERNAME}
ENV ARTIFACTORY_PASSWORD=${ARTIFACTORY_PASSWORD}

RUN gem source -a https://${ARTIFACTORY_USERNAME}:${ARTIFACTORY_PASSWORD}@usw1.packages.broadcom.com/artifactory/api/gems/tas-rel-eng-gem-dev-local/
RUN gem install --verbose ops-manifest -v 3.3.2

RUN which ops-manifest

RUN printf '%s\n%s\n' \
    'registry=https://${ARTIFACTORY_USERNAME}:${ARTIFACTORY_PASSWORD}@usw1.packages.broadcom.com/artifactory/api/npm/tis-npm-virtual/' \
    'always-auth=true' > /root/.npmrc
