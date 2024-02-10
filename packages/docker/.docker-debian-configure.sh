#!/usr/bin/env sh
# This script attempts to install sq via apt, yum, apk, or brew.
# Parts of the script are inspired by the get-docker.sh
# script at https://get.docker.com

set -e

#apt-get update -y && apt-get install bash-completion -y
#apt-get update -y

cat <<EOF >> /etc/bash.bashrc
# enable bash completion in interactive shells
if ! shopt -oq posix; then
  if [ -f /usr/share/bash-completion/bash_completion ]; then
    . /usr/share/bash-completion/bash_completion
  elif [ -f /etc/bash_completion ]; then
    . /etc/bash_completion
  fi
fi
EOF
# enable bash completion in interactive shells
#if ! shopt -oq posix; then
#  if [ -f /usr/share/bash-completion/bash_completion ]; then
#    . /usr/share/bash-completion/bash_completion
#  elif [ -f /etc/bash_completion ]; then
#    . /etc/bash_completion
#  fi
#fi




printf "Using apt to install sq...\n\n"

apt update -y && apt install -y --no-upgrade curl gpg wget jq bash bash-completion zsh git
chsh -s $(which zsh) $USER
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"


cat <<EOF >> /etc/bash.bashrc
# enable bash completion in interactive shells
if ! shopt -oq posix; then
  if [ -f /usr/share/bash-completion/bash_completion ]; then
    . /usr/share/bash-completion/bash_completion
  elif [ -f /etc/bash_completion ]; then
    . /etc/bash_completion
  fi
fi
EOF



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

sq config set log true
sq config set log.file /work/sq.log
sq config set shell-completion.log true



