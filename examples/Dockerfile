FROM golang:1.17 as server

WORKDIR /docker4ssh

COPY ["../", "."]

RUN apt update && \
    apt install make sqlite3 && \
    apt clean && \
    apt autoremove && \
    rm -rf /var/lib/apt/lists/*

RUN make BUILDDIR=build/ build-server


FROM rust:1.56 as client

WORKDIR /docker4ssh

COPY ../ .

RUN apt update && \
    apt install make \

RUN make BUILDDIR=build/ build-client


FROM alpine:lastest as extra

WORKDIR /docker4ssh

COPY ../ .

RUN apk add make

RUN make BUILDDIR=build/ build-extra


FROM alpine:latest

WORKDIR /docker4ssh

COPY --from=server /docker4ssh/build/* .
COPY --from=client /docker4ssh/build/docker4ssh .
COPY --from=extra /docker4ssh/build/* .

ENTRYPOINT docker4ssh
