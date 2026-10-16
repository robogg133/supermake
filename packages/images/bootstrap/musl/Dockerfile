# Static musl-targeting toolchain (binutils + gcc) built on a glibc host, then
# merged onto the base image so the toolchain binaries run regardless of libc.
FROM gcc:16.2.0 AS build
WORKDIR /src

ADD --unpack=true https://musl.libc.org/releases/musl-1.2.6.tar.gz .
WORKDIR /src/musl-1.2.6
RUN ./configure --prefix=/usr
RUN make -j8
RUN make install DESTDIR=/stage

WORKDIR /src
ADD --unpack=true https://ftp.gnu.org/gnu/binutils/binutils-2.45.tar.gz .
WORKDIR /src/binutils-2.45
RUN ./configure --prefix=/usr --libdir=/usr/lib --build=$(sh config.guess) --target=x86_64-linux-musl --with-sysroot=/ --disable-nls --disable-shared --disable-gprofng --disable-werror
RUN make -j8
RUN make install DESTDIR=/stage
# musl-prefixed binutils must be on PATH for the gcc build below
RUN cp /stage/usr/bin/x86_64-linux-musl-* /usr/bin/

WORKDIR /src
ADD --unpack=true https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-16.2.0/gcc-16.2.0.tar.gz .
WORKDIR /src/gcc-16.2.0
RUN ./contrib/download_prerequisites
RUN ./configure --prefix=/usr --libdir=/usr/lib --build=$(sh config.guess) --target=x86_64-linux-musl --with-sysroot=/ --with-build-sysroot=/stage --disable-nls --disable-shared --disable-bootstrap --disable-multilib --enable-languages=c,c++ --disable-lto --disable-libsanitizer --disable-libquadmath --disable-libgomp CFLAGS=-O2 CXXFLAGS=-O2 LDFLAGS=-static
RUN make -j8
RUN make install DESTDIR=/stage

FROM supermake/base-musl
COPY --from=build /stage /
RUN ln -s x86_64-linux-musl-gcc /usr/bin/gcc \
 && ln -s x86_64-linux-musl-g++ /usr/bin/g++ \
 && ln -s x86_64-linux-musl-cpp /usr/bin/cpp \
 && ln -s x86_64-linux-musl-as /usr/bin/as \
 && ln -s x86_64-linux-musl-ld /usr/bin/ld \
 && ln -s x86_64-linux-musl-ar /usr/bin/ar \
 && ln -s x86_64-linux-musl-nm /usr/bin/nm \
 && ln -s x86_64-linux-musl-ranlib /usr/bin/ranlib \
 && ln -s x86_64-linux-musl-strip /usr/bin/strip \
 && ln -s x86_64-linux-musl-objcopy /usr/bin/objcopy \
 && ln -s x86_64-linux-musl-objdump /usr/bin/objdump \
 && ln -s x86_64-linux-musl-readelf /usr/bin/readelf \
 && ln -s x86_64-linux-musl-c++filt /usr/bin/c++filt
