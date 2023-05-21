# mysql backup image
FROM alpine:3.17
LABEL org.opencontainers.image.authors="https://github.com/deitch"

# If you're in China, set IN_CHINA to true.
ARG IN_CHINA=true

RUN set -xe; \
    if [ "${CHANGE_SOURCE}" = true ] || [ "${IN_CHINA}" = true ]; then \
    sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/' /etc/apk/repositories; \
    fi

# install the necessary client
# the mysql-client must be 10.3.15 or later
RUN apk add --no-cache 'mariadb-client>10.3.15' mariadb-connector-c bash python3 py3-pip samba-client shadow openssl coreutils dumb-init && \
    rm -rf /var/cache/apk/* && \
    touch /etc/samba/smb.conf && \
    python3 -m pip install --no-cache-dir awscli

# set us up to run as non-root user
RUN groupadd -g 1000 appuser && \
    useradd -r -u 1000 -g appuser appuser
# ensure smb stuff works correctly
RUN mkdir -p /var/cache/samba && chmod 0755 /var/cache/samba && chown appuser /var/cache/samba && chown appuser /var/lib/samba/private
USER appuser

# install the entrypoint
COPY functions.sh /
COPY entrypoint /entrypoint

# start
ENTRYPOINT [ "/usr/bin/dumb-init", "--" ]
CMD ["/entrypoint"]
