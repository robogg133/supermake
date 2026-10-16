# Static binutils + gcc built on the glibc host, plus a glibc dev tree from
# source (2.41, same release as the glibc shipped in busybox:1.38.0-glibc),
# merged onto the base image.
FROM gcc:16.2.0 AS build
WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends bison

# glibc dev tree: headers, crt objects, static and shared libs; OUT of source
ADD --unpack=true https://ftp.gnu.org/gnu/glibc/glibc-2.41.tar.xz .
WORKDIR /src/glibc-2.41
RUN mkdir build
WORKDIR /src/glibc-2.41/build
RUN ../configure --prefix=/usr
RUN make -j8
RUN make install DESTDIR=/stage slibdir=/usr/lib/x86_64-linux-gnu libdir=/usr/lib/x86_64-linux-gnu
RUN rm -f /stage/etc/ld.so.cache /stage/etc/ld.so.conf*

WORKDIR /src
ADD --unpack=true https://ftp.gnu.org/gnu/binutils/binutils-2.45.tar.gz .
WORKDIR /src/binutils-2.45
RUN ./configure --prefix=/usr --libdir=/usr/lib --disable-nls --disable-shared --disable-gprofng --disable-werror
RUN make -j8
RUN make install DESTDIR=/stage
# make the native-static binutils shadow Debian's on PATH for the gcc build
RUN cp /stage/usr/bin/* /usr/bin/

WORKDIR /src
ADD --unpack=true https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-16.2.0/gcc-16.2.0.tar.gz .
WORKDIR /src/gcc-16.2.0
RUN ./contrib/download_prerequisites
RUN ./configure --prefix=/usr --libdir=/usr/lib --disable-nls --disable-shared --disable-bootstrap --disable-multilib --enable-languages=c,c++ --disable-lto --disable-libsanitizer --disable-libquadmath --disable-libgomp CFLAGS=-O2 CXXFLAGS=-O2 LDFLAGS=-static
RUN make -j8
RUN make install DESTDIR=/stage

FROM supermake/base-glibc
COPY --from=build /stage /
RUN ln -s gcc /usr/bin/cc
