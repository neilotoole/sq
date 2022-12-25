#!/usr/bin/env sh
# This script attempts to install sq via apt, yum, apk, or brew.
# Parts of the script are inspired by the get-docker.sh
# script at https://get.docker.com


get_distribution() {
	lsb_dist=""
	# Every system that we officially support has /etc/os-release
	if [ -r /etc/os-release ]; then
		lsb_dist="$(. /etc/os-release && echo "$ID")"
	fi
	# Returning an empty string here should be alright since the
	# case statements don't act unless you provide an actual value
	echo "$lsb_dist"
}

# Usage:
#
# if command_exists lsb_release; then
command_exists() {
	command -v "$@" > /dev/null 2>&1
}

get_distribution

# apt / deb
if [ -r /etc/debian_version ] && command_exists apt; then
  set -e
  printf "Using apt to install sq...\n\n"

  apt update -y && apt install -y --no-upgrade curl gpg

  curl -fsSL https://apt.fury.io/neilotoole/gpg.key | gpg --dearmor -o /usr/share/keyrings/sq.gpg

  echo "deb [signed-by=/usr/share/keyrings/sq.gpg] https://apt.fury.io/neilotoole/ * *" > /etc/apt/sources.list.d/sq.list

  cat <<EOF > /etc/apt/preferences.d/sq
Package: sq
Pin: origin apt.fury.io
Pin-Priority: 501
EOF

  apt update -y && apt install -y sq

  printf "\n"
  sq version
  printf "\n"
  exit
fi


# Yum / rpm
if command_exists yum; then
  set -e
  set +x
  printf "Using yum to install sq...\n\n"

  cat <<EOF > /etc/yum.repos.d/sq.repo
[sq]
name=sq
baseurl=https://yum.fury.io/neilotoole/
enabled=1
gpgcheck=0
gpgkey=https://apt.fury.io/neilotoole/gpg.key
EOF

  yum install -y sq

  printf "\n"
  sq version
  printf "\n"
  exit
fi


# apk / alpine
if command_exists apk; then
  set -e
  printf "Using apk to install sq...\n\n"
  apk update

  # sq isn't published to an Alpine repo yet, so we download the
  # file from GitHub, and execute "apk add" with the local apk file.

  # e.g. "v1.0.0"
  semver=$(wget -qO- "https://api.github.com/repos/neilotoole/sq/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

  # e.g. "1.0.0"
  ver=$(echo "$semver" | sed -e "s/^v//")

  # Should be "x86_64" for amd64, and "aarch64" for arm64
  arch=$(uname -m)

  if [ "$arch" == "x86_64" ]; then
    arch="amd64"
  elif [ "$arch" == "aarch64" ]; then
    arch="arm64"
  else
    printf "sq install package not available for architecture %q\n" $arch
    exit 1
  fi

  # e.g. "sq_0.18.1_linux_arm64.apk"
  file_name=$(printf "sq_%s_linux_%s.apk" "$ver" $arch)
  file_path="/tmp/$file_name"

  # https://github.com/neilotoole/sq/releases/download/v0.18.1/sq_0.18.1_linux_amd64.apk
  # https://github.com/neilotoole/sq/releases/download/v0.18.1/sq_0.18.1_linux_arm64.apk
  download_url=$(printf "https://github.com/neilotoole/sq/releases/download/%s/%s" "$semver" "$file_name")

  echo "Downloading apk from: $download_url"
  wget  "$download_url" -O "$file_path"

  apk add --allow-untrusted "$file_path"
  rm "$file_path"

  printf "\n"
  sq version
  printf "\n"
  exit
fi

# brew
if command_exists brew; then
  set -e
  printf "Using brew to install sq...\n\n"

  brew install neilotoole/sq/sq

  printf "\n"
  sq version
  printf "\n"
  exit
fi


printf "\nCould not find a suitable install mechanism to install sq.\n"
printf "\nVisit https://github.com/neilotoole/sq for more installation options.\n"
exit 1




