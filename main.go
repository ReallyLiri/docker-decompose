package main

import (
	"fmt"
	"github.com/reallyliri/docker-decompose/decompose"
	"github.com/urfave/cli/v2"
	"log"
	"os"
)

const (
	Version = "0.1.1"
)

func declareCli() *cli.App {
	cli.AppHelpTemplate =
		`NAME:
   {{.Name}} - {{.Version}} - {{.Usage}}
USAGE:
   {{.Name}} {{if .Flags}}[Options]{{end}} [compose-file] [service, ...]
ARGS:
    compose-file{{ "\t" }}path to a docker-compose.yaml file, defaults to docker-compose.yaml at current directory
    service(s){{ "\t" }}zero or more service names to decompose, defaults to all services in the compose file
OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}
`
	return &cli.App{
		Name:    "docker-decompose",
		Version: Version,
		Usage:   "Decompose docker compose files to docker build and run commands",
		Writer:  os.Stdout,
		Flags:   decompose.Flags(),
	}
}

func main() {
	app := declareCli()
	app.Action = func(context *cli.Context) error {
		opts := decompose.ParseOptions(context)
		commands, err := decompose.Decompose(opts)
		if err != nil {
			return err
		}
		for _, command := range commands {
			fmt.Printf(command)
			fmt.Print("\n\n")
		}
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("run failed: %v", err)
	}
}
