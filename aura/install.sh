#!/usr/bin/env sh

# Hmmn

# If root, then we need to create a builduser
pacman -S --needed --noconfirm sudo base-devel

# If not root, then it's easier
