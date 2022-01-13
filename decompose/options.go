package decompose

import (
	"errors"
	"fmt"
	"github.com/urfave/cli/v2"
	"log"
	"os"
)

const envDefaultPath = ".env"

type Options struct {
	composeFilePath   string
	serviceNames      []string
	getBuildCommand   bool
	getRunCommand     bool
	getNetworkCommand bool
	envFilePath       string
	restart           string
	skipEnvInherit    bool
}

func Flags() []cli.Flag {
	return []cli.Flag{

		&cli.BoolFlag{
			Name:  "no-build",
			Usage: "Skip printing docker-build commands",
		},

		&cli.BoolFlag{
			Name:  "no-run",
			Usage: "Skip printing docker-run commands",
		},

		&cli.BoolFlag{
			Name:  "no-network",
			Usage: "Skip printing docker-network-create commands",
		},

		&cli.BoolFlag{
			Name:  "no-env-inherit",
			Usage: "Don't pass on external environment variables",
		},

		&cli.StringFlag{
			Name:    "env",
			Aliases: []string{"e"},
			Value:   envDefaultPath,
			Usage:   "Path to env file to apply when rendering compose, will be skipped if does not exist",
		},

		&cli.StringFlag{
			Name:    "restart",
			Aliases: []string{"r"},
			Value:   "",
			Usage:   "Restart flag to pass to docker-run command, one of [no, always, on-failure, unless-stopped]. If not specified, will be taken from compose",
		},
	}
}

func ParseOptions(context *cli.Context) (opts *Options) {
	args := context.Args()
	opts = &Options{
		composeFilePath:   "docker-compose.yaml",
		serviceNames:      []string{},
		getBuildCommand:   true,
		getRunCommand:     true,
		getNetworkCommand: true,
		skipEnvInherit:    false,
		envFilePath:       envDefaultPath,
	}
	if args.Len() >= 1 {
		opts.composeFilePath = args.Get(0)
	}
	for i := 1; i < args.Len(); i++ {
		opts.serviceNames = append(opts.serviceNames, args.Get(i))
	}
	if _, err := os.Stat(opts.composeFilePath); errors.Is(err, os.ErrNotExist) {
		log.Fatalf("docker compose file does not exist at '%v'", opts.composeFilePath)
	}
	opts.getBuildCommand = !context.Bool("no-build")
	opts.getRunCommand = !context.Bool("no-run")
	opts.getNetworkCommand = !context.Bool("no-network")
	opts.skipEnvInherit = context.Bool("no-env-inherit")
	opts.envFilePath = context.String("env")
	if _, err := os.Stat(opts.envFilePath); errors.Is(err, os.ErrNotExist) {
		if opts.envFilePath != envDefaultPath {
			fmt.Printf("env file does not exist at '%v'\n", opts.envFilePath)
		}
		opts.envFilePath = ""
	}
	if opts.restart != "" && !allowedRestartValues[opts.restart] {
		fmt.Printf("invalid value for restart flag, defaulting to compose config\n")
		opts.restart = ""
	}
	return
}
