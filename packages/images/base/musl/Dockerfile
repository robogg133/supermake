# base-musl
FROM gcc:16.2.0 AS build
WORKDIR /src

ADD --unpack=true https://ftp.gnu.org/gnu/make/make-4.4.1.tar.gz .
WORKDIR /src/make-4.4.1

RUN ./configure LDFLAGS=-static
RUN make -j8

FROM busybox:1.38.0-musl
COPY --from=build /src/make-4.4.1/make /bin/make
