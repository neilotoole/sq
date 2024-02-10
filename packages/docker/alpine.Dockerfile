FROM alpine:3.19
WORKDIR /work

RUN apk add --update --no-cache bash bash-completion curl git tzdata wget jq coreutils utmps
COPY ./.docker-alpine-configure.sh ./.docker-alpine-configure.sh
RUN chmod +x ./.docker-alpine-configure.sh
RUN ./.docker-alpine-configure.sh

COPY ./entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]

