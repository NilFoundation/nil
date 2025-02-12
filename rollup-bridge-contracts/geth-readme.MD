## Build Docker Image

```sh
docker build -t geth-local .
```

## Run Geth Node

```sh
docker-compose up -d
```

## get the docker image name and id

```sh
docker ps -a
```

## check docker logs

```sh
docker logs --follow <image-name>
```
