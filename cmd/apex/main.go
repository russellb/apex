package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc/jsonrpc"
	"os"

	"github.com/urfave/cli/v2"
)

// This variable is set using ldflags at build time. See Makefile for details.
var Version = "dev"

func callApex(method string) (string, error) {
	conn, err := net.Dial("unix", "/run/apex.sock")
	if err != nil {
		fmt.Printf("Failed to connect to apexd: %+v", err)
		return "", err
	}
	defer conn.Close()

	client := jsonrpc.NewClient(conn)

	var result string
	err = client.Call("ApexCtl."+method, nil, &result)
	if err != nil {
		fmt.Printf("Failed to execute method (%s): %+v", method, err)
		return "", err
	}
	return result, nil
}

func checkVersion() error {
	result, err := callApex("Version")
	if err != nil {
		fmt.Printf("Failed to get apexd version: %+v\n", err)
		return err
	}

	if Version != result {
		errMsg := fmt.Sprintf("Version mismatch: apex(%s) apexd(%s)\n", Version, result)
		fmt.Print(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

func cmdVersion(cCtx *cli.Context) error {
	fmt.Printf("apex version: %s\n", Version)

	result, err := callApex("Version")
	if err == nil {
		fmt.Printf("apexd version: %s\n", result)
	}
	return err
}

func cmdStatus(cCtx *cli.Context) error {
	if err := checkVersion(); err != nil {
		return err
	}

	result, err := callApex("Status")
	if err == nil {
		fmt.Print(result)
	}
	return err
}

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:    "version",
				Aliases: []string{"v"},
				Usage:   "Get the version of apex and apexd",
				Action:  cmdVersion,
			},
			{
				Name:    "status",
				Aliases: []string{"s"},
				Usage:   "Get the status of apexd",
				Action:  cmdStatus,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
