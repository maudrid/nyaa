#!/bin/bash
go build nyaa.go
docker build -t nyaa:latest .
