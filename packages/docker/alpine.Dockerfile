FROM alpine:3.19
WORKDIR /root

RUN apk add --update --no-cache curl gnupg wget bash bash-completion zsh \
    tzdata git jq coreutils utmps vim nano \
    sqlite postgresql-client mysql-client


# Microsoft SQL Server Tools are slightly tricker to install.
COPY alpine-mssql-tools-install.sh ./mssql-tools-install.sh
RUN chmod +x ./alpine-mssql-tools-install.sh
RUN ./alpine-mssql-tools-install.sh
ENV PATH=$PATH:/opt/mssql-tools18/bin

COPY alpine-install-sq.sh ./alpine-install-sq.sh
RUN chmod +x ./alpine-install-sq.sh
RUN ./alpine-install-sq.sh

COPY sq.zsh-theme ./sq.zsh-theme
COPY configure-oh-my-zsh.sh ./configure-oh-my-zsh.sh
RUN chmod +x ./configure-oh-my-zsh.sh
RUN ./configure-oh-my-zsh.sh


RUN rm ./alpine-install-sq.sh  ./alpine-mssql-tools-install.sh ./configure-oh-my-zsh.sh


COPY ./entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
