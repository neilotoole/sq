#!/usr/bin/env sh
# This script adds the "sq" repo to apt, but does not install "sq".
# To install, run

apt update -y && apt install -y curl gpg

curl -fsSL https://apt.fury.io/neilotoole/gpg.key | gpg --dearmor -o /usr/share/keyrings/sq.gpg

echo "deb [signed-by=/usr/share/keyrings/sq.gpg] https://apt.fury.io/neilotoole/ * *" > /etc/apt/sources.list.d/sq.list

cat <<EOF > /etc/apt/preferences.d/sq
Package: sq
Pin: origin apt.fury.io
Pin-Priority: 501
EOF

apt update -y
