#FROM golang:1.21-alpine as builder
#
#RUN apk add --update --no-cache bash curl git make tzdata wget jq build-base sqlite-dev
#
#WORKDIR /work

## Fetch dependencies first; they are less susceptible to change on every build
## and will therefore be cached for speeding up the next build
#COPY ./go.mod ./go.sum ./
#RUN go mod download
#
## Note that .dockerignore excludes things we don't want to copy.
#COPY ./ ./
#RUN echo "huzzah `pwd`"
#RUN ls -alF
#RUN echo "$GOPATH"
#RUN go install github.com/goreleaser/goreleaser@v1.20.0
##ENV CGO_ENABLED=1
#
#
#RUN make install
#RUN go install
#RUN which sq
#RUN cp ./bin/sq /bin

FROM debian:bullseye
WORKDIR /work
# RUN #apk add --no-cache add tzdata bash curl git wget jq
#RUN #apk add --update --no-cache bash bash-completion curl git tzdata wget jq
COPY ./.docker-debian-configure.sh ./.docker-debian-configure.sh
RUN chmod +x ./.docker-debian-configure.sh
RUN ./.docker-debian-configure.sh
#COPY --from=builder /bin/sq /bin/

#CMD ["/bin/bash"]
#ENTRYPOINT ["/bin/bash"]
#COPY ./entrypoint.sh /entrypoint.sh
#RUN #chmod +x /entrypoint.sh
#ENTRYPOINT ["/bin/bash" , "/entrypoint.sh"]
