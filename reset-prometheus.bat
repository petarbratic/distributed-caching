@echo off

docker compose restart backend gateway-1 gateway-2

docker compose stop prometheus
docker compose rm -f -v prometheus
docker compose up -d --force-recreate prometheus

echo.
echo Backend, gateway instances and Prometheus have been reset.
pause