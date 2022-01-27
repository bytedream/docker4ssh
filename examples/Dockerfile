FROM alpine:latest as build

WORKDIR /docker4ssh

RUN apk add git go make openssh rustup sqlite

RUN rustup-init -y && \
    ln -s $HOME/.cargo/bin/cargo /usr/bin/cargo

RUN git clone https://github.com/ByteDream/docker4ssh .

RUN mkdir build/ && \
    make BUILDDIR=build/ build && \
    rm -rf build/release/ build/x86_64-unknown-linux-musl/

FROM alpine:latest

WORKDIR /docker4ssh

COPY --from=build /docker4ssh/build/ ./

ENTRYPOINT ./docker4ssh start
