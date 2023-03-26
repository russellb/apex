package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nexodus-io/nexodus/internal/nexodus"
	"github.com/urfave/cli/v2"

	"go.uber.org/zap"
)

const (
	nexodusLogEnv = "NEXTUN_LOGLEVEL"
	nextunBinary  = "nextun"
)

// This variable is set using ldflags at build time. See Makefile for details.
var Version = "dev"

func main() {
	// set the log level
	debug := os.Getenv(nexodusLogEnv)
	var logger *zap.Logger
	var err error
	if debug != "" {
		logger, err = zap.NewDevelopment()
		logger.Info("Debug logging enabled")
	} else {
		logCfg := zap.NewProductionConfig()
		logCfg.DisableStacktrace = true
		logger, err = logCfg.Build()
	}
	if err != nil {
		logger.Fatal(err.Error())
	}

	// Overwrite usage to capitalize "Show"
	cli.HelpFlag.(*cli.BoolFlag).Usage = "Show help"
	// flags are stored in the global flags variable
	app := &cli.App{
		Name:      nextunBinary,
		Usage:     "Tunnel connections over a Nexodus network.",
		ArgsUsage: "controller-url",
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Get the version of " + nextunBinary,
				Action: func(cCtx *cli.Context) error {
					fmt.Printf("version: %s\n", Version)
					return nil
				},
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "public-key",
				Value:    "",
				Usage:    "Public key for the local host - agent generates keys by default",
				EnvVars:  []string{"NEXTUN_PUB_KEY"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "private-key",
				Value:    "",
				Usage:    "Private key for the local host (dev purposes only - soon to be removed)",
				EnvVars:  []string{"NEXTUN_PRIVATE_KEY"},
				Required: false,
			},
			&cli.IntFlag{
				Name:     "listen-port",
				Value:    0,
				Usage:    "Port wireguard is to listen for incoming peers on",
				EnvVars:  []string{"NEXTUN_LISTEN_PORT"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "request-ip",
				Value:    "",
				Usage:    "Request a specific IP address from Ipam if available (optional)",
				EnvVars:  []string{"NEXTUN_REQUESTED_IP"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "local-endpoint-ip",
				Value:    "",
				Usage:    "Specify the endpoint address of this node instead of being discovered (optional)",
				EnvVars:  []string{"NEXTUN_LOCAL_ENDPOINT_IP"},
				Required: false,
			},
			&cli.StringSliceFlag{
				Name:     "child-prefix",
				Usage:    "Request a CIDR range of addresses that will be advertised from this node (optional)",
				EnvVars:  []string{"NEXTUN_REQUESTED_CHILD_PREFIX"},
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "stun",
				Usage:    "Discover the public address for this host using STUN",
				Value:    false,
				EnvVars:  []string{"NEXTUN_STUN"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "username",
				Value:    "",
				Usage:    "Username for accessing the nexodus service",
				EnvVars:  []string{"NEXTUN_USERNAME"},
				Required: false,
			},
			&cli.StringFlag{
				Name:     "password",
				Value:    "",
				Usage:    "Password for accessing the nexodus service",
				EnvVars:  []string{"NEXTUN_PASSWORD"},
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "insecure-skip-tls-verify",
				Value:    false,
				Usage:    "If true, server certificates will not be checked for validity. This will make your HTTPS connections insecure",
				EnvVars:  []string{"NEXTUN_INSECURE_SKIP_TLS_VERIFY"},
				Required: false,
			},
			&cli.StringSliceFlag{
				Name:     "ingress",
				Usage:    "Forward connections from the Nexodus network made to [port] on this proxy instance to port [destination_port] at [destination_ip] via a locally accessible network using a `value` in the form: protocol:port:destination_ip:destination_port. All fields are required.",
				Required: false,
			},
			&cli.StringSliceFlag{
				Name:     "egress",
				Usage:    "Forward connections from a locally accessible network made to [port] on this proxy instance to port [destination_port] at [destination_ip] via the Nexodus network using a `value` in the form: protocol:port:destination_ip:destination_port. All fields are required.",
				Required: false,
			},
		},
		Action: func(cCtx *cli.Context) error {

			controller := cCtx.Args().First()
			if controller == "" {
				logger.Info("<controller-url> required")
				return nil
			}

			ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
			nex, err := nexodus.NewNexodus(
				ctx,
				logger.Sugar(),
				controller,
				cCtx.String("username"),
				cCtx.String("password"),
				cCtx.Int("listen-port"),
				cCtx.String("public-key"),
				cCtx.String("private-key"),
				cCtx.String("request-ip"),
				cCtx.String("local-endpoint-ip"),
				cCtx.StringSlice("child-prefix"),
				cCtx.Bool("stun"),
				false, // relay-node
				false, // discovery-node
				false, // relay-only
				cCtx.Bool("insecure-skip-tls-verify"),
				Version, true,
			)
			if err != nil {
				logger.Fatal(err.Error())
			}

			// for validating that connectivity is working, to be removed later
			go func() {
				time.Sleep(time.Second * 10)
				for {
					client := http.Client{
						Transport: &http.Transport{
							DialContext: nex.UserspaceNet.DialContext,
						},
					}
					resp, err := client.Get("http://100.100.0.1/")
					if err != nil {
						log.Panic(err)
					}
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						log.Panic(err)
					}
					log.Println(string(body))
					time.Sleep(time.Second * 5)
				}
			}()

			wg := &sync.WaitGroup{}
			for _, egressRule := range cCtx.StringSlice("egress") {
				err := nex.UserspaceProxyAdd(ctx, wg, egressRule, nexodus.ProxyTypeEgress)
				if err != nil {
					logger.Sugar().Errorf("Failed to add egress proxy rule (%s): %v", egressRule, err)
				}
			}
			for _, ingressRule := range cCtx.StringSlice("ingress") {
				err := nex.UserspaceProxyAdd(ctx, wg, ingressRule, nexodus.ProxyTypeIngress)
				if err != nil {
					logger.Sugar().Errorf("Failed to add egress proxy rule (%s): %v", ingressRule, err)
				}
			}

			if err := nex.Start(ctx, wg); err != nil {
				logger.Fatal(err.Error())
			}
			wg.Wait()

			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		logger.Fatal(err.Error())
	}
}
