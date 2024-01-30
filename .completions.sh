#!/bin/sh
# ".completions.sh" regenerates shell completions to "./completions".
# This script is invoked by a hook in .goreleaser.yml during the CI
# release process, generating "sq.bash", "sq.zsh", etc. into "./completions".
# goreleaser then uses those generate completions for the various install
# packages it creates.
#
# Note that the "./completions" dir is not committed to git; it is generated
# on demand when goreleaser runs.

set -e

rm -rf completions
mkdir completions

for sh in bash zsh fish powershell; do
	go run main.go completion "$sh" >"completions/sq.$sh"
done
