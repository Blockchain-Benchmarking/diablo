docker-compose -f host1.yaml down -v
docker rm $(docker ps -aq)

