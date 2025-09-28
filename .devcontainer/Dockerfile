FROM ubuntu:24.04

# Avoid prompts from apt
ENV DEBIAN_FRONTEND=noninteractive

# Install Go and essential tools in one layer to reduce image size
RUN apt-get update && apt-get install -y --no-install-recommends \
    golang-go \
    git \
    make \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Set working directory (Codespaces uses /workspaces/{repo-name})
WORKDIR /workspaces

# Use the existing ubuntu user (UID 1000) instead of creating a new one
USER ubuntu

# Set Go environment
ENV GO111MODULE=on \
    GOPATH=/home/ubuntu/go \
    PATH=/home/ubuntu/go/bin:/usr/bin:$PATH