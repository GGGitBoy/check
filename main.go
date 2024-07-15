package main

import (
	"fmt"
	"inspection-server/pkg/agent"
	"inspection-server/pkg/db"
	"inspection-server/pkg/schedule"
	"inspection-server/pkg/server"
	"inspection-server/pkg/template"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	VERSION = "dev"
	port    int
)

func main() {
	app := cli.NewApp()
	app.Name = "inspection"
	app.Version = VERSION
	app.Usage = "Provides one-touch inspection capability for clusters."
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:        "port",
			EnvVar:      "HTTP_PORT",
			Value:       8080,
			Usage:       "The inspection listen port.",
			Destination: &port,
		},
		cli.StringFlag{
			Name:   "serverUrl",
			EnvVar: "SERVER_URL",
			Usage:  "The server url of rancher.",
		},
		cli.StringFlag{
			Name:   "bearerToken",
			EnvVar: "BEARER_TOKEN",
			Usage:  "The bearer token of rancher.",
		},
		cli.BoolFlag{
			Name:   "debug",
			EnvVar: "LOG_LEVEL",
			Usage:  "Set log level to debug.",
		},
		cli.BoolFlag{
			Name:   "mysql",
			EnvVar: "MY_SQL",
			Usage:  "Set mysql as database.",
		},
		cli.StringFlag{
			Name:   "mysqlUser",
			EnvVar: "MY_SQL_USER",
			Value:  "root",
			Usage:  "Set mysql user.",
		},
		cli.StringFlag{
			Name:   "mysqlPassword",
			EnvVar: "MY_SQL_PASSWORD",
			Value:  "123456",
			Usage:  "Set mysql password.",
		},
		cli.StringFlag{
			Name:   "mysqlHost",
			EnvVar: "MY_SQL_HOST",
			Value:  "localhost",
			Usage:  "Set mysql host.",
		},
		cli.IntFlag{
			Name:   "mysqlPort",
			EnvVar: "MY_SQL_PORT",
			Value:  3306,
			Usage:  "Set mysql port.",
		},
		cli.StringFlag{
			Name:   "mysqlDB",
			EnvVar: "MY_SQL_DB",
			Value:  "mysql",
			Usage:  "Set mysql database.",
		},
		cli.StringFlag{
			Name:   "systemDefaultRegistry",
			EnvVar: "SYSTEM_DEFAULT_REGISTRY",
			Value:  "",
			Usage:  "Set system default registry.",
		},
	}

	app.Action = func(ctx *cli.Context) error {
		router := server.Start()
		logrus.Infof("[Main] Server running, listening at: %d\n", port)

		err := http.ListenAndServe(fmt.Sprintf(":%d", port), router)
		if err != nil {
			logrus.Errorf("Failed to start server: %v", err)
			return err
		}
		return nil
	}
	app.Before = before

	if err := app.Run(os.Args); err != nil {
		logrus.Fatalf("Application encountered an error: %v", err)
	}
}

func before(ctx *cli.Context) error {
	if ctx.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("Debug mode enabled")
	}

	err := db.Register()
	if err != nil {
		logrus.Errorf("Failed to register database: %v", err)
		return err
	}

	err = schedule.Register()
	if err != nil {
		logrus.Errorf("Failed to register schedule: %v", err)
		return err
	}

	err = template.Register()
	if err != nil {
		logrus.Errorf("Failed to register template: %v", err)
		return err
	}

	err = agent.Register()
	if err != nil {
		logrus.Errorf("Failed to register agent: %v", err)
		return err
	}

	return nil
}
