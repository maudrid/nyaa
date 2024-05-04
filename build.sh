#!/bin/bash
go build nyaa.go
docker build -t nyaa:latest .
docker save nyaa:latest -o ./nyaa.tar
