package decompose

import (
	"github.com/bmizerany/assert"
	"github.com/compose-spec/godotenv"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func setup(t *testing.T, content string, env map[string]string) *Options {
	composePath := filepath.Join(t.TempDir(), "docker-compose.yaml")
	err := ioutil.WriteFile(composePath, []byte(content), 0644)
	require.Nil(t, err)
	envPath := ""
	if env != nil {
		envPath = filepath.Join(t.TempDir(), ".env")
		err = godotenv.Write(env, envPath)
		require.Nil(t, err)
	}
	return &Options{
		composeFilePath:   composePath,
		serviceNames:      []string{},
		envFilePath:       envPath,
		getRunCommand:     true,
		getBuildCommand:   true,
		getNetworkCommand: true,
	}
}

func TestDecomposeSimple(t *testing.T) {
	//language=yaml
	content := `
version: "3.9"
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
`
	opts := setup(t, content, nil)
	defer os.Remove(opts.composeFilePath)

	commands, err := Decompose(opts)
	require.Nil(t, err)
	assert.Equal(t, 3, len(commands))
	assert.Equal(t, `docker build -f "Dockerfile" -t "web" .`, commands[0])
	assert.Equal(t, `docker run -d --name "web" -e "FLASK_ENV=development" -p "5000:5000" -v ".:/code" "web"`, commands[1])
	assert.Equal(t, `docker run -d --name "redis" "redis:alpine"`, commands[2])
}

func TestDecomposeComplex(t *testing.T) {
	//language=yaml
	content := `
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
`
	opts := setup(t, content, map[string]string{"BRANCH": "dev", "COLL_TAG": "l4t3st"})
	opts.restart = "unless-stopped"
	defer os.Remove(opts.composeFilePath)
	defer os.Remove(opts.envFilePath)

	commands, err := Decompose(opts)
	require.Nil(t, err)
	assert.Equal(t, 5, len(commands))
	assert.Equal(t, `docker network create db-net`, commands[0])
	assert.Equal(t, `docker build -f "Dockerfile" -t "collectiwise/main:l4t3st" .`, commands[1])
	assert.Equal(t, `docker run -d --name "postgres_triple" -e "POSTGRES_PASSWORD=postgres" --network "db-net" --restart "unless-stopped" -v "/var/pgdata/triple:/var/lib/postgresql/data" "postgres:9.5"`, commands[2])
	assert.Equal(t, `docker run -d --name "postgres" -e "POSTGRES_PASSWORD=postgres" --network "db-net" --restart "unless-stopped" -v "pgdata:/var/lib/postgresql/data" "postgres:9.5"`, commands[3])
	assert.Equal(t, `docker run -d --name "collectiwise" -e "COLLECTIWISE_BRANCH=dev" --network "db-net" -p "8090:80" --restart "unless-stopped" "collectiwise/main:l4t3st"`, commands[4])
}
