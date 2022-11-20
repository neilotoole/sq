#!/usr/bin/env sh
# This script attempts to install sq via apt, yum, or brew.


# Test if apt is installed
apt --version >/dev/null 2>&1
if [ "$?" -eq "0" ]; then
  set -e
  echo "Using apt to install sq..."

  apt update -y && apt install -y --no-upgrade curl gpg

  curl -fsSL https://apt.fury.io/neilotoole/gpg.key | gpg --dearmor -o /usr/share/keyrings/sq.gpg

  echo "deb [signed-by=/usr/share/keyrings/sq.gpg] https://apt.fury.io/neilotoole/ * *" > /etc/apt/sources.list.d/sq.list

  cat <<EOF > /etc/apt/preferences.d/sq
Package: sq
Pin: origin apt.fury.io
Pin-Priority: 501
EOF

  apt update -y && apt install -y sq

  exit
fi


# Test if yum is installed
yum version >/dev/null 2>&1
if [ "$?" -eq "0" ]; then
  set -e
  echo "Using yum to install sq..."

  cat <<EOF > /etc/yum.repos.d/sq.repo
[sq]
name=sq
baseurl=https://yum.fury.io/neilotoole/
enabled=1
gpgcheck=0
gpgkey=https://apt.fury.io/neilotoole/gpg.key
EOF

  yum install -y sq

  exit
fi


# Test if brew is installed
brew --version >/dev/null 2>&1
if [ "$?" -eq "0" ]; then
  set -e
  echo "Using brew to install sq..."

  brew install neilotoole/sq/sq

  exit
fi


echo ""
echo "Could not find a suitable install mechanism to install sq."
echo ""
echo "Visit https://github.com/neilotoole/sq for more installation options."
exit 1



