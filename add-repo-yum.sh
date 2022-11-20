#!/usr/bin/env sh
# This script adds the "sq" repo to yum.

cat <<EOF > /etc/yum.repos.d/sq.repo
[sq]
name=sq
baseurl=https://yum.fury.io/neilotoole/
enabled=1
gpgcheck=0
gpgkey=https://apt.fury.io/neilotoole/gpg.key
EOF

