#!/usr/bin/env bash
# Generate CMD.help.txt file for each sq command,
# using "sq CMD --help > CMD.help.txt". The generated
# help text is included from the corresponding markdown
# file.

set -e

# Always execute in this dir
cd $(dirname "$0")


# For each option, we want to get the help text for that option
# as returned by "sq config set OPT --help".
#
# First, we get the names of each option from "sq config ls -jv" piped
# to jq to get the key. Those keys go into the array opt_keys.
IFS=$'\n' read -r -d '' -a opt_keys < <( sq config ls -jv | jq -r '.[] | .key' && printf '\0' )

rm -f ./options/*.help.txt

# For each element of opt_keys, generate the help text.
for opt in "${opt_keys[@]}"; do
  dest="./options/${opt}.help.txt"

  # Trim the last two lines, as they always contain a message linking
  # to the sq.io site: those lines are superfluous.
  sq config set "$opt" --help | head -n -2 > "$dest"
done
