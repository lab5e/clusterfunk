#!/bin/sh -e
cp ../bin/demo.linux demo
docker build demo --tag clusterfunk-demo
