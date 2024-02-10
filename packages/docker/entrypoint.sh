#!/usr/bin/env sh

if [ $# -eq 0 ]; then
  # No arguments supplied, e.g. "docker run -t -d neilotoole/sq".
  # We want to keep the container running, so we sleep for infinity.
  # https://stackoverflow.com/questions/30209776/docker-container-will-automatically-stop-after-docker-run-d
  # https://stackoverflow.com/questions/2935183/bash-infinite-sleep-infinite-blocking/45396600#45396600
  sleep infinity
else
  # Arguments supplied, e.g. "docker run -it neilotoole/sq bash"
  exec "$@"
fi


