FROM golang:1.7.1

RUN apt-get update && apt-get install -y --no-install-recommends jq tar zip
RUN rm -rf /var/lib/apt/lists/*

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

ENV REPOPATH $GOPATH/src/github.com/neilotoole/sq
RUN mkdir -p "$REPOPATH"
ADD . "$REPOPATH"
WORKDIR $REPOPATH

RUN make install-go-tools