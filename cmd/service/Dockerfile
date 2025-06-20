#---------------------------------------------------------------------------------------------------
# go-blog-big-data [Dockerfile]
# 
# Reference: https://stackoverflow.com/questions/63178036/how-to-find-commit-hash-from-within-a-running-docker-image
# commit: git rev-parse HEAD
# 
# https://stackoverflow.com/questions/6245570/how-to-get-the-current-branch-name-in-git
# branch: git rev-parse --abbrev-ref HEAD
# 
# https://gist.github.com/asukakenji/f15ba7e588ac42795f421b48b8aede63
# The GOOS will always be linux, but in case you were curious
# 
# Sample docker build commands:
#  docker build -f ./cmd/Dockerfile . -t ghcr.io/antonio-alexander/go-blog-big-data:amd64_latest --build-arg GIT_COMMIT=$GITHUB_SHA --build-arg GIT_BRANCH=$GITHUB_REF --build-arg PLATFORM=linux/amd64 --build-arg GO_ARCH=amd64
#  docker build -f ./cmd/Dockerfile . -t ghcr.io/antonio-alexander/go-blog-big-data:armv7_latest --build-arg GIT_COMMIT=$GITHUB_SHA --build-arg GIT_BRANCH=$GITHUB_REF --build-arg PLATFORM=linux/arm/v7 --build-arg GO_ARCH=arm --build-arg GO_ARM=7
# 
#---------------------------------------------------------------------------------------------------

ARG GIT_BRANCH=no_git_branch_provided
ARG GIT_COMMIT=no_git_commit_provided
ARG PLATFORM=linux/amd64
ARG GO_ARCH=amd64
ARG GO_ARM=7

FROM golang:alpine AS build

ARG GIT_BRANCH
ARG GIT_COMMIT
ARG GO_ARCH
ARG GO_ARM

RUN apk add jq

WORKDIR /go/src/go-blog-big-data

COPY ./go.mod /go/src/go-blog-big-data/go.mod

RUN go mod download

COPY . /go/src/go-blog-big-data

RUN \
    VERSION=`cat /go/src/go-blog-big-data/version.json | jq '.version' | sed 's/"//g'` \
    cd cmd/service \
    && env GOARCH=${GO_ARCH} GOARM=${GO_ARM} GOOS=linux go build -ldflags \
    "-X github.com/antonio-alexander/go-blog-big-data/internal/data.Version=${VERSION} \
    -X github.com/antonio-alexander/go-blog-big-data/internal/data.GitCommit=${GIT_COMMIT} \
    -X github.com/antonio-alexander/go-blog-big-data/internal/data.GitBranch=${GIT_BRANCH}" \
    -o go-blog-big-data \
    && chmod +X go-blog-big-data \
    && tar -czvf go-blog-big-data.tar.gz ./go-blog-big-data

FROM --platform=${PLATFORM} alpine:latest

ARG GIT_BRANCH
ARG GIT_COMMIT

WORKDIR /app

COPY --from=build /go/src/go-blog-big-data/cmd/service/go-blog-big-data.tar.gz /app/go-blog-big-data.tar.gz

LABEL antonio-alexander.git.branch=${GIT_BRANCH}
LABEL antonio-alexander.git.commit=${GIT_COMMIT}
LABEL org.opencontainers.image.source=https://github.com/antonio-alexander/go-blog-big-data

CMD tar -xzf go-blog-big-data.tar.gz && ./go-blog-big-data