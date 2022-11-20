#!/usr/bin/env bash

apt update -y
sudo add-apt-repository -y ppa:longsleep/golang-backports
sudo apt update -y
sudo apt install -y golang-1.19

echo "export PATH=$PATH:/usr/lib/go-1.19/bin" >> ~/.bashrc
echo "export GOPATH=$HOME/go" >> ~/.bashrc
