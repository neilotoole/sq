#!/bin/sh
# ".completions.sh regenerates completions to "./completions".

set -e
rm -rf completions
mkdir completions
for sh in bash zsh fish powershell; do
	go run main.go completion "$sh" >"completions/sq.$sh"
done
