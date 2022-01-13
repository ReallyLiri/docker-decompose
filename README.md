# docker-decompose

Tool to convert `docker-compose` files to set of simple `docker` commands.

![icon](icon.png)

## Install

Use `go get` to install the latest version of the library.

```bash
go get -u github.com/reallyliri/docker-decompose
```

## Usage

```
NAME:
   docker-decompose - Decompose docker compose files to docker build and run commands
USAGE:
   docker-decompose [Options] [compose-file] [service, ...]
ARGS:
    compose-file  path to a docker-compose.yaml file, defaults to docker-compose.yaml at current directory
    service(s)    zero or more service names to decompose, defaults to all services in the compose file
OPTIONS:
   --no-build                 Skip printing docker-build commands (default: false)
   --no-run                   Skip printing docker-run commands (default: false)
   --no-network               Skip printing docker-network-create commands (default: false)
   --no-env-inherit           Don't pass on external environment variables (default: false)
   --env value, -e value      Path to env file to apply when rendering compose, will be skipped if does not exist (default: ".env")
   --restart value, -r value  Restart flag to pass to docker-run command, one of [no, always, on-failure, unless-stopped]. If not specified, will be taken from compose
   --help, -h                 show help (default: false)
   --version, -v              print the version (default: false)
```

### Examples

```bash
docker-decompose my-compose.yaml svc-1 svc-2
docker-decompose --restart unless-stopped --env ~/.env
```

Output for given compose files:

---------------
```yaml
version: "3"
services:
  web:
    build: .
    ports:
      - "5000:5000"
    volumes:
      - .:/code
    environment:
      FLASK_ENV: development
  redis:
    image: "redis:alpine"
```

===>

```bash
docker build -f "Dockerfile" -t "web" .

docker run -n "web" -e "FLASK_ENV=development" -p "5000:5000" -v ".:/code" "web"

docker run -n "redis" "redis:alpine"
```

---------------

```yaml
version: "3"
services:
    postgres_triple:
        container_name: postgres_triple
        image: postgres:9.5
        volumes:
            - pgdata_triple:/var/lib/postgresql/data
        environment:
            - POSTGRES_PASSWORD=postgres
        networks:
            - db-net
    postgres:
        container_name: postgres
        image: postgres:9.5
        volumes:
            - pgdata:/var/lib/postgresql/data
        environment:
            - POSTGRES_PASSWORD=postgres
        networks:
            - db-net
    collectiwise:
        container_name: collectiwise
        build: .
        image: collectiwise/main:${COLL_TAG}
        ports:
            - "8090:80"
        environment:
            - COLLECTIWISE_BRANCH=${BRANCH}
        networks:
            - db-net
volumes:
    pgdata:
    pgdata_triple:
      driver_opts:
        o: bind
        type: none
        device: "/var/pgdata/triple"

networks:
  db-net:
```

with `.env` file containing `BRANCH=dev COLL_TAG=l4t3st`

===>

```bash
docker network create db-net

docker build -f "Dockerfile" -t "collectiwise/main:l4t3st" .

docker run -n "collectiwise" -e "COLLECTIWISE_BRANCH=dev" --network "db-net" -p "8090:80" "collectiwise/main:l4t3st"

docker run -n "postgres_triple" -e "POSTGRES_PASSWORD=postgres" --network "db-net" -v "/var/pgdata/triple:/var/lib/postgresql/data" "postgres:9.5"

docker run -n "postgres" -e "POSTGRES_PASSWORD=postgres" --network "db-net" -v "pgdata:/var/lib/postgresql/data" "postgres:9.5"
```
