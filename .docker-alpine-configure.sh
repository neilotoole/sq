#!/usr/bin/env sh
# This script attempts to install sq via apt, yum, apk, or brew.
# Parts of the script are inspired by the get-docker.sh
# script at https://get.docker.com



# apk / alpine
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

if [ "$arch" = "x86_64" ]; then
  arch="amd64"
elif [ "$arch" = "aarch64" ]; then
  arch="arm64"
else
  printf "sq install package not available for architecture: %s\n" "$arch"
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
source /etc/bash/bash_completion.sh
echo "source /etc/bash/bash_completion.sh" >> /etc/bash/bashrc
mkdir -p /etc/bash_completion.d/
#mkdir -p "$HOME/.config/bash_completion"
sq completion bash > /etc/bash_completion.d/sq
exit


