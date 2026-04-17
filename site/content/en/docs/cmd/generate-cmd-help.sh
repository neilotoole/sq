#!/usr/bin/env bash
# Generate CMD.help.txt file for each sq command,
# using "sq CMD --help > CMD.help.txt". The generated
# help text is included from the corresponding markdown
# file.

set -e

# Always execute in this dir
cd $(dirname "$0")

cmds=(
  "add"
  "completion"
  "completion bash"
  "completion zsh"
  "completion fish"
  "completion powershell"
  "cache clear"
  "cache disable"
  "cache enable"
  "cache location"
  "cache stat"
  "cache tree"
  "config location"
  "config ls"
  "config get"
  "config set"
  "config edit"
  "diff"
  "driver ls"
  "group"
  "help"
  "inspect"
  "ls"
  "mv"
  "ping"
  "rm"
  "sql"
  "src"
  "tbl copy"
  "tbl drop"
  "tbl truncate"
  "version"
   )

rm -f ./*.help.txt
rm -f ./*.output.txt

for cmd in "${cmds[@]}"; do
  # space -> dash, e.g. "driver ls" -> "driver-ls"
  dest="${cmd// /-}.help.txt"

  # shellcheck disable=SC2086
  sq $cmd --help > "$dest"
done

# Special handling for the root command.
sq --help > sq.help.txt

# Show output for some commands
sq driver ls > driver-ls.output.txt


# For each option, we want to get the help text for that option
# as returned by "sq config set OPT --help", and output that help
# text into a file in the ./options dir.
#
# First, we get the names of each option from "sq config ls -jv" piped
# to jq to get the key. Those keys go into the array opt_keys.
IFS=$'\n' read -r -d '' -a opt_keys < <( sq config ls -jv | jq -r '.[] | .key' && printf '\0' )

mkdir -p ./options
rm -f ./options/*.help.txt

# For each element of opt_keys, generate the help text.
for opt in "${opt_keys[@]}"; do
  dest="./options/${opt}.help.txt"

  # Trim the first two lines, as they contain generic text.
  # Also trim the last two lines, as they always contain a message linking
  # to the sq.io site: those lines are superfluous.
  sq config set "$opt" --help | tail -n +3 | head -n -2 > "$dest"
done
