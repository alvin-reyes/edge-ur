# Delta Dockerfile
# Description: Dockerfile for delta
# This is the multi-stage docker image to build and run delta
# Author: Outercore Engineering
# Name: delta
# Email:
# Url: https://delta.store

FROM golang:1.19 AS builder

RUN apt-get update && \
    apt-get install -y wget jq hwloc ocl-icd-opencl-dev git libhwloc-dev pkg-config make && \
    apt-get install -y cargo

WORKDIR /app/
ADD . /app

RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"
RUN RUSTFLAGS="-C target-cpu=native -g" FFI_BUILD_FROM_SOURCE=1 FFI_USE_BLST_PORTABLE=1 make

FROM golang:1.19

ARG REPO="/root/config/.whypfs"

RUN echo "Building docker image for edge"

RUN apt-get update && \
    apt-get install -y hwloc libhwloc-dev ocl-icd-opencl-dev
WORKDIR /root/

COPY --from=builder /app/edge ./
CMD ./edge daemon --repo=${REPO}
EXPOSE 1313