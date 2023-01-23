package apex

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"sync"

	"github.com/redhat-et/apex/internal/util"
)

// TODO make this path configurable
const UnixSocketPath = "/run/apex.sock"

func (ax *Apex) CtlServerStart(ctx context.Context, wg *sync.WaitGroup) error {
	switch ax.os {
	case Linux.String():
		ax.CtlServerLinuxStart(ctx, wg)

	case Darwin.String():
		ax.logger.Info("Ctl interface not yet supported on OSX")

	case Windows.String():
		ax.logger.Info("Ctl interface not yet supported on Windows")
	}

	return nil
}

type ApexCtl struct {
	ax *Apex
}

func (ac *ApexCtl) Status(_ string, result *string) error {
	var statusStr string
	switch ac.ax.status {
	case ApexStatusStarting:
		statusStr = "Starting"
	case ApexStatusAuth:
		statusStr = "WaitingForAuth"
	case ApexStatusRunning:
		statusStr = "Running"
	default:
		statusStr = "Unknown"
	}
	res := fmt.Sprintf("Status: %s\n", statusStr)
	if len(ac.ax.statusMsg) > 0 {
		res += ac.ax.statusMsg
	}
	*result = res
	return nil
}

func (ac *ApexCtl) Version(_ string, result *string) error {
	*result = ac.ax.version
	return nil
}

func (ax *Apex) CtlServerLinuxStart(ctx context.Context, wg *sync.WaitGroup) {
	util.GoWithWaitGroup(wg, func() {
		for {
			// Use a different waitgroup here, because we want to make sure
			// all of the subroutines have exited before we attempt to restart
			// the control server.
			ctlWg := &sync.WaitGroup{}
			err := ax.CtlServerLinuxRun(ctx, ctlWg)
			ctlWg.Done()
			if err == nil {
				// No error means it shut down cleanly because it got a message to stop
				break
			}
			ax.logger.Error("Ctl interface error, restarting: ", err)
		}
	})
}

func (ax *Apex) CtlServerLinuxRun(ctx context.Context, ctlWg *sync.WaitGroup) error {
	os.Remove(UnixSocketPath)
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: UnixSocketPath, Net: "unix"})
	if err != nil {
		ax.logger.Error("Error creating unix socket: ", err)
		return err
	}

	ac := new(ApexCtl)
	ac.ax = ax
	err = rpc.Register(ac)
	if err != nil {
		ax.logger.Error("Error on rpc.Register(): ", err)
		l.Close()
		return err
	}

	// We accept connections and send them back over this channel
	// so we can select waiting on new connections and a message
	// to stop the CtlServer at the same time. This routine will
	// exit when the listener is closed intentionally, or some
	// error occurs.
	connChan := make(chan net.Conn)
	errChan := make(chan error)
	util.GoWithWaitGroup(ctlWg, func() {
		for {
			conn, err := l.Accept()
			connChan <- conn
			if conn == nil {
				errChan <- err
				break
			}
		}
	})

	// Handle new connections until we get notified to stop the CtlServer,
	// or Accept() fails for some reason.
	stopNow := false
	for {
		select {
		case conn := <-connChan:
			if conn == nil {
				// Accept() failed, collect the error and stop the CtlServer
				stopNow = true
				err = <-errChan
				ax.logger.Error("Error on Accept(): ", err)
				break
			}
			util.GoWithWaitGroup(ctlWg, func() {
				jsonrpc.ServeConn(conn)
			})
		case <-ctx.Done():
			ax.logger.Info("Stopping CtlServer")
			// This will cause the Accept() routine to exit
			l.Close()
			stopNow = true
			err = nil
		}
		if stopNow {
			break
		}
	}

	return err
}
