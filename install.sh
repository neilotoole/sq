#!/usr/bin/env sh
# This script attempts to install sq via apt, yum, or brew.

get_latest_release() {
  echo "in there"
  curl --silent "https://api.github.com/repos/neilotoole/sq/releases/latest" | # Get latest release from GitHub api
    grep '"tag_name":' |                                            # Get tag line
    sed -E 's/.*"([^"]+)".*/\1/'                                    # Pluck JSON value
}




echo "huzzah"

# Test if apt is installed
apk --version >/dev/null 2>&1
if [ "$?" -eq "0" ]; then
  set -e
  printf "\nUsing apk to install sq...\n\n"

  if ! command -v curl &> /dev/null; then
    apk update && apk add curl
  fi

  if ! command -v dpkg &> /dev/null; then
    apk update && apk add dpkg
  fi

  # e.g. "v1.0.0"
  semver=$(curl -s "https://api.github.com/repos/neilotoole/sq/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

  # e.g. "1.0.0"
  ver=$(echo "$semver" | sed -e "s/^v//")

  # e.g. "sq_1.0.0_linux_arm64.deb
  deb=$(printf "sq_%s_linux_arm64.deb" "$ver")


  echo "semver: $semver  ver:  $ver  deb: $deb"
  download_url=$(printf "https://github.com/neilotoole/sq/releases/download/%s/%s" "$semver" "$deb")

  curl -s -o /tmp/"$deb" "$download_url"

  dpkg -i /tmp/"$deb"

  sq version

# https://git.alpinelinux.org/aports/tree/community/hugo/APKBUILD

  rm /tmp/"$deb"

#wget https://dl.influxdata.com/influxdb/releases/influxdb_0.13.0_armhf.deb
#sudo dpkg -i influxdb_0.13.0_armhf.deb
  # apk install dpkg

  exit
fi




## Test if apt is installed
#apt --version >/dev/null 2>&1
#if [ "$?" -eq "0" ]; then
#  set -e
#  echo "Using apt to install sq..."
#  echo ""
#
#  apt update -y && apt install -y --no-upgrade curl gpg
#
#  curl -fsSL https://apt.fury.io/neilotoole/gpg.key | gpg --dearmor -o /usr/share/keyrings/sq.gpg
#
#  echo "deb [signed-by=/usr/share/keyrings/sq.gpg] https://apt.fury.io/neilotoole/ * *" > /etc/apt/sources.list.d/sq.list
#
#  cat <<EOF > /etc/apt/preferences.d/sq
#Package: sq
#Pin: origin apt.fury.io
#Pin-Priority: 501
#EOF
#
#  apt update -y && apt install -y sq
#
#  exit
#fi
#
#
## Test if yum is installed
#yum version >/dev/null 2>&1
#if [ "$?" -eq "0" ]; then
#  set -e
#  echo "Using yum to install sq..."
#  echo ""
#
#  cat <<EOF > /etc/yum.repos.d/sq.repo
#[sq]
#name=sq
#baseurl=https://yum.fury.io/neilotoole/
#enabled=1
#gpgcheck=0
#gpgkey=https://apt.fury.io/neilotoole/gpg.key
#EOF
#
#  yum install -y sq
#
#  exit
#fi
#
#
## Test if brew is installed
#brew --version >/dev/null 2>&1
#if [ "$?" -eq "0" ]; then
#  set -e
#  echo "Using brew to install sq..."
#  echo ""
#
#  brew install neilotoole/sq/sq
#
#  exit
#fi


echo ""
echo "Could not find a suitable install mechanism to install sq."
echo ""
echo "Visit https://github.com/neilotoole/sq for more installation options."
exit 1



