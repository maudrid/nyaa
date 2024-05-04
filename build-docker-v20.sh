#!/bin/bash
go build nyaa.go
docker run -tid --rm --privileged --name docker-builder -v $(pwd):/nyaa docker:20.10.24-dind
sleep 5
docker exec -ti docker-builder docker build -t nyaa:latest /nyaa
docker exec -ti docker-builder docker save nyaa:latest -o /nyaa/nyaa.tar
docker stop docker-builder
