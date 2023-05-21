#!/bin/sh

set -xe

if [ "${CHANGE_SOURCE}" = true ] || [ "${IN_CHINA}" = true ]; then
    sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/' /etc/apk/repositories
fi

# curl ca-certificates && rm -rf /var/cache/apk/*

# install the necessary client
# the mysql-client must be 10.3.15 or later
apk update
apk add --no-cache --update 'mariadb-client>10.3.15'  dumb-init \
    mariadb-connector-c bash python3 py3-pip samba-client shadow openssl coreutils
rm -rf /var/cache/apk/*
touch /etc/samba/smb.conf

# pip install -i https://pypi.tuna.tsinghua.edu.cn/simple some-package
# pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple
# pip config set global.extra-index-url "<url1> <url2>..."
python3 -m pip install -i https://mirrors.ustc.edu.cn/pypi/web/simple pip -U
python3 -m pip config set global.extra-index-url "https://mirrors.ustc.edu.cn/pypi/web/simple https://pypi.tuna.tsinghua.edu.cn/simple"
python3 -m pip install --no-cache-dir awscli

# set us up to run as non-root user
groupadd -g 1000 appuser
useradd -u 1000 -r -g appuser appuser

mkdir /backup
chown 1000:1000 /backup
# ensure smb stuff works correctly
mkdir -p /var/cache/samba
chmod 0755 /var/cache/samba
chown appuser /var/cache/samba
chown appuser /var/lib/samba/private

chmod +x /*.sh
