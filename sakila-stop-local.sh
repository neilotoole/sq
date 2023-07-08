#!/usr/bin/env bash

# This script stops the containers started by sakila-start-local.sh

set +e

docker rm -f sakiladb-pg
docker rm -f sakiladb-az
docker rm -f sakiladb-my
