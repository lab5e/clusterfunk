# Docker-stack for demo cluster

This directory contains a docker-composek setup + build script for the a docker
image containing the demo server.

## Build demo server image

Run `sh build-demo-container.sh` to build a docker image

## Run docker stack

Run `docker-compose up -d` to launch a two-node cluster. Scale up with
`docker-compose scale node=4` to increase the size.