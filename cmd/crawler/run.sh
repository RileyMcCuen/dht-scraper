#/bin/bash
GOARCH=amd64 GOOS=linux go build .
scp crawler root@147.182.235.141:/root/crawler
ssh root@147.182.235.141 "./crawler"