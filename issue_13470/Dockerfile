FROM golang:1.5.2

MAINTAINER @tschottdorf

RUN apt-get update && \
 apt-get install --no-install-recommends --auto-remove -y git vim emacs libc-dbg gdb strace

RUN go get -d github.com/tschottdorf/goplay

WORKDIR /go/src/github.com/tschottdorf/goplay/issue_13470

RUN git clone --branch glibc-2.19 --depth 1 git://sourceware.org/git/glibc.git

RUN echo 'set auto-load safe-path /' >> ~/.gdbinit
RUN echo 'set directories glibc/nis' >> ~/.gdbinit
RUN go build -o boom -ldflags '-extldflags "-static"' main.go

CMD ["/bin/bash"]
