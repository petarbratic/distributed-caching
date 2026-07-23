@echo off

docker compose restart gateway

docker compose stop prometheus
docker compose rm -f -v prometheus
docker compose up -d prometheus