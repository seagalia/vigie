package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/seagalia/vigie/app"
	"github.com/seagalia/vigie/config"
)

var (
	version = "0.0.1-dev"
	commit  = "none"
	date    = "unknown"
)

type ServeCmd struct {
	ConfigPath string `required:"" short:"c" help:"path to the configuration file" type:"path" default:"./config.yml"`
}

func (r *ServeCmd) Run() error {
	cnf, err := config.ReadConfig(r.ConfigPath)
	if err != nil {
		return fmt.Errorf("read config: %s", err)
	}
	appRun, err := app.NewApp(cnf, r.ConfigPath, version)
	if err != nil {
		return fmt.Errorf("create app: %s", err)
	}
	return appRun.Run()
}

type CheckConfigCmd struct {
	ConfigPath string `required:"" short:"c" help:"path to the configuration file" type:"path" default:"./config.yml"`
}

func (r *CheckConfigCmd) Run() error {
	_, err := config.ReadConfigStrict(r.ConfigPath)
	if err != nil {
		return fmt.Errorf("invalid config %s: %s", r.ConfigPath, err)
	}
	fmt.Printf("config %s is valid\n", r.ConfigPath)
	return nil
}

type VersionCmd struct {
}

func (l *VersionCmd) Run() error {
	fmt.Println(versionLine())
	return nil
}

func versionLine() string {
	return fmt.Sprintf("vigie %s, commit %s, built at %s", version, commit, date)
}

type CLI struct {
	Serve       ServeCmd       `cmd:"" help:"Run the endpoint discovery server."`
	CheckConfig CheckConfigCmd `cmd:"" name:"check-config" help:"Check a configuration file, unknown keys included, without running the server."`
	Version     VersionCmd     `cmd:"" help:"Show version."`
}

func newParser(c *CLI, options ...kong.Option) (*kong.Kong, error) {
	baseOptions := []kong.Option{
		kong.Name("vigie"),
		kong.Description("Endpoint discovery server: services from DNS, a file or a URL, served as JSON over HTTP."),
		// the full help of all the commands would bury the error
		kong.ShortUsageOnError(),
	}
	return kong.New(c, append(baseOptions, options...)...)
}

func main() {
	c := &CLI{}
	parser, err := newParser(c)
	if err != nil {
		panic(err)
	}
	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)
	ctx.FatalIfErrorf(ctx.Run())
}
