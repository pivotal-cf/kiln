# syntax=docker/dockerfile:1.10

FROM golang AS go-image
FROM pivotalcfreleng/kiln:v1.0.0 AS kiln

FROM ruby:3.4.0 AS builder
RUN mkdir -p -m 0700 ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts

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
RUN go install github.com/onsi/ginkgo/v2/ginkgo@latest

# Install NodeJS
RUN apt-get update && apt-get install nodejs npm -y

# Install OpsManifest from Artifactory
ARG ARTIFACTORY_USERNAME
ARG ARTIFACTORY_PASSWORD

ENV ARTIFACTORY_USERNAME=${ARTIFACTORY_USERNAME}
ENV ARTIFACTORY_PASSWORD=${ARTIFACTORY_PASSWORD}

RUN gem source -a https://${ARTIFACTORY_USERNAME}:${ARTIFACTORY_PASSWORD}@usw1.packages.broadcom.com/artifactory/api/gems/tas-rel-eng-gem-dev-local/
RUN gem install --verbose ops-manifest -v 0.0.3.pre

RUN which ops-manifest
