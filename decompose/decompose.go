package decompose

import (
	"fmt"
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"github.com/compose-spec/godotenv"
	"github.com/wesovilabs/koazee"
	"io/ioutil"
	"os"
	"strings"
)

var allowedRestartValues = map[string]bool{
	"no":             true,
	"always":         true,
	"on-failure":     true,
	"unless-stopped": true,
}

type context struct {
	includeServiceNames   []string
	serviceDependencies   map[string]map[string]bool
	buildCommandsByName   map[string]string
	runCommandsByName     map[string]string
	networkCommandsByName map[string]string
	usedNetworks          map[string]bool
	restartFlag           string
	volumeNameToPath      map[string]string
	project               *types.Project
}

func Decompose(opts *Options) (commands []string, err error) {
	project, err := load(opts)
	if err != nil {
		return nil, err
	}
	ctx := &context{
		serviceDependencies:   map[string]map[string]bool{},
		buildCommandsByName:   map[string]string{},
		runCommandsByName:     map[string]string{},
		networkCommandsByName: map[string]string{},
		usedNetworks:          map[string]bool{},
		restartFlag:           opts.restart,
		volumeNameToPath:      map[string]string{},
		includeServiceNames:   opts.serviceNames,
		project:               project,
	}
	err = ctx.parseNetworks()
	if err != nil {
		return nil, err
	}
	err = ctx.parseVolumes()
	if err != nil {
		return nil, err
	}
	err = ctx.parseServices()

	commands = []string{}
	if opts.getNetworkCommand {
		for network := range ctx.usedNetworks {
			command := ctx.networkCommandsByName[network]
			if command != "" {
				commands = append(commands, command)
			}
		}
	}

	for _, serviceName := range ctx.sortedServices() {
		command := ctx.buildCommandsByName[serviceName]
		if command != "" {
			commands = append(commands, command)
		}
		command = ctx.runCommandsByName[serviceName]
		if command != "" {
			commands = append(commands, command)
		}
	}

	return commands, nil
}

func load(opts *Options) (*types.Project, error) {
	env := map[string]string{}
	var err error
	if opts.envFilePath != "" {
		env, err = godotenv.Read(opts.envFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load env from '%v': %v", opts.envFilePath, err)
		}
	}
	if !opts.skipEnvInherit {
		envList := os.Environ()
		for _, pair := range envList {
			parts := strings.Split(pair, "=")
			env[parts[0]] = parts[1]
		}
	}
	content, err := ioutil.ReadFile(opts.composeFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose from '%v': %v", opts.composeFilePath, err)
	}
	project, err := loader.Load(types.ConfigDetails{
		Version:    "3",
		WorkingDir: ".",
		ConfigFiles: []types.ConfigFile{
			{
				Filename: opts.composeFilePath,
				Content:  content,
			},
		},
		Environment: env,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose from '%v': %v", opts.composeFilePath, err)
	}
	return project, nil
}

func (ctx *context) parseNetworks() error {
	for network := range ctx.project.Networks {
		command := fmt.Sprintf("docker network create %v", network)
		ctx.networkCommandsByName[network] = command
	}
	return nil
}

func (ctx *context) parseVolumes() error {
	for name, volumeConfig := range ctx.project.Volumes {
		ctx.volumeNameToPath[name] = name
		device := volumeConfig.DriverOpts["device"]
		if device != "" {
			ctx.volumeNameToPath[name] = device
		}
	}
	return nil
}

func (ctx *context) parseServices() error {
	serviceNamesStream := koazee.StreamOf(ctx.includeServiceNames)

	for _, service := range ctx.project.Services {
		serviceName := service.Name
		if len(ctx.includeServiceNames) > 0 {
			contains, err := serviceNamesStream.Contains(serviceName)
			if err != nil {
				return fmt.Errorf("failed to check service names: %v", err)
			}
			if !contains {
				continue
			}
		}

		imageName := serviceName
		if service.Image != "" {
			imageName = service.Image
		} else if service.ContainerName != "" {
			imageName = service.ContainerName
		}

		build := service.Build
		if build != nil {
			buildCommand := ctx.constructBuildCommand(build, imageName)
			if buildCommand != "" {
				ctx.buildCommandsByName[serviceName] = buildCommand
			}
		}

		runCommand := ctx.constructRunCommand(&service, imageName)
		if runCommand != "" {
			ctx.runCommandsByName[serviceName] = runCommand
		}

		ctx.serviceDependencies[serviceName] = map[string]bool{}
		for name := range service.DependsOn {
			ctx.serviceDependencies[serviceName][name] = true
		}
	}

	return nil
}

func (ctx *context) constructBuildCommand(build *types.BuildConfig, imageName string) string {
	builder := &strings.Builder{}
	builder.WriteString("docker build ")
	if build.Dockerfile != "" {
		writeFlagArg(builder, "-f", build.Dockerfile)
	}
	writeFlagArg(builder, "-t", imageName)
	for name, value := range build.Args {
		writeFlagArg(builder, "--build-arg", formatKeyValue(name, value, "="))
	}
	for _, cacheFrom := range build.CacheFrom {
		writeFlagArg(builder, "--cache-from", cacheFrom)
	}
	if build.Target != "" {
		writeFlagArg(builder, "--target", build.Target)
	}
	buildContext := build.Context
	if buildContext == "" {
		buildContext = "."
	}
	builder.WriteString(buildContext)
	return builder.String()
}

func (ctx *context) constructRunCommand(service *types.ServiceConfig, imageName string) string {
	builder := &strings.Builder{}
	builder.WriteString("docker run ")

	writeFlagArg(builder, "-n", service.Name)

	if len(service.Entrypoint) > 0 {
		builder.WriteString("--entrypoint ")
		writeShellCommand(service.Entrypoint, builder)
	}

	for name, value := range service.Environment {
		writeFlagArg(builder, "-e", formatKeyValue(name, value, "="))
	}

	for name := range service.Networks {
		if name == "default" {
			continue
		}
		writeFlagArg(builder, "--network", name)
		ctx.usedNetworks[name] = true
	}

	for _, port := range service.Ports {
		formatted := fmt.Sprintf("%v:%v", port.Published, port.Target) // host:container
		writeFlagArg(builder, "-p", formatted)
	}

	restartFlag := ctx.restartFlag
	if restartFlag == "" {
		restartFlag = service.Restart
	}
	if restartFlag != "" {
		writeFlagArg(builder, "--restart", restartFlag)
	}

	if service.Hostname != "" {
		writeFlagArg(builder, "-h", service.Hostname)
	}

	for _, volume := range service.Volumes {
		source := volume.Source
		target := volume.Target
		if volume.Type == "volume" {
			source = ctx.volumeNameToPath[source]
		}
		if source != "" && target != "" {
			writeFlagArg(builder, "-v", formatKeyValue(source, &target, ":"))
		}
	}

	builder.WriteString(`"`)
	builder.WriteString(imageName)
	builder.WriteString(`"`)

	if len(service.Command) > 0 {
		builder.WriteString(" ")
		writeShellCommand(service.Command, builder)
	}

	return builder.String()
}

func writeFlagArg(builder *strings.Builder, flag string, arg string) {
	builder.WriteString(flag)
	builder.WriteString(` "`)
	builder.WriteString(arg)
	builder.WriteString(`" `)
}

func writeShellCommand(shellCommand types.ShellCommand, builder *strings.Builder) {
	builder.WriteString(`"`)
	for i, command := range shellCommand {
		if i > 0 {
			builder.WriteString(" ")
		}
		builder.WriteString(command)
	}
	builder.WriteString(`"`)
}

func formatKeyValue(name string, value *string, operator string) string {
	formatted := name
	if value != nil {
		formatted = fmt.Sprintf("%v%v%v", name, operator, *value)
	}
	return formatted
}

func (ctx *context) sortedServices() (sorted []string) {
	visitedServices := map[string]bool{}
	for len(ctx.serviceDependencies) > 0 {
		for serviceName, dependencies := range ctx.serviceDependencies {
			for dependency := range dependencies {
				if visitedServices[dependency] || ctx.serviceDependencies[dependency] == nil {
					delete(dependencies, dependency)
				}
			}
			if len(dependencies) == 0 {
				sorted = append(sorted, serviceName)
				visitedServices[serviceName] = true
				delete(ctx.serviceDependencies, serviceName)
			}
		}
	}
	return
}
