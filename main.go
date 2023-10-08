package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"

	"github.com/alecthomas/kong"
	log "github.com/sirupsen/logrus"
)

var cli struct {
	LogLevel         string `enum:"trace,debug,info,warn,error,fatal,panic" help:"The log level to log with" default:"info"`
	ListenAddr       string `name:"listen-address" help:"The path of the unix domain socket to listen on" arg:""`
	FirstRemoteAddr  string `name:"first-remote-address" help:"The first (and authoritative) socket address to forward requests to"`
	SecondRemoteAddr string `name:"second-remote-address" help:"The second socket address responses are compared against"`
}

func main() {
	_ = kong.Parse(&cli, kong.Description("Duplicate requests received over unix sockets and send them to two backends"))

	logLevel, err := log.ParseLevel(cli.LogLevel)
	if err != nil {
		log.Fatal("invalid log level")
	}
	log.SetLevel(logLevel)

	log.Infof("Listening on %s", cli.ListenAddr)

	l, err := net.Listen("unix", cli.ListenAddr)
	if err != nil {
		log.WithError(err).Fatal("failed to listen")
	}

	// chmod the socket file to be world-accessible
	if err := os.Chmod(cli.ListenAddr, 0777); err != nil {
		log.WithError(err).Fatal("failed to chmod socket")
	}

	conns := make(chan net.Conn, 32)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Spin up a goroutine that keeps accepting new connections, and inserting them
	// into a channel until the channel buffer is full.
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				log.WithError(err).Warn("failed to accept")
			}

			conns <- conn
		}
	}()

	// Main loop
	for {
		select {
		case conn := <-conns:
			defer conn.Close()

			if err := handleConn(conn); err != nil {
				log.WithError(err).Error("error handling connection")
			}
		case <-ctx.Done():
			log.Info("Received interrupt, terminating")

			if err := os.Remove(cli.ListenAddr); err != nil {
				log.WithError(err).Warn("failed to remove listening socket")
			}
			return
		}
	}
}

func handleConn(conn net.Conn) error {
	// read up to 4096 bytes
	b := make([]byte, 4096)

	n, err := conn.Read(b)
	if err != nil {
		log.WithError(err).Error("reading from client")
	}

	rq := b[:n]

	log := log.WithFields(log.Fields{
		"remote_addr": conn.RemoteAddr().String(),
		"rq":          fmt.Sprintf("%X", rq),
		"rq_len":      n,
	})

	log.Debug("received request")

	// connect to first backend.
	firstConn, err := net.Dial("unix", cli.FirstRemoteAddr)
	if err != nil {
		return fmt.Errorf("unable to connect to first backend: %w", err)
	}
	defer firstConn.Close()

	// send the request to the first backend.
	if _, err := io.Copy(firstConn, bytes.NewReader(rq)); err != nil {
		return fmt.Errorf("failed writing to first backend: %w", err)
	}

	// read the response.
	var resp1 bytes.Buffer
	if _, err := io.Copy(&resp1, firstConn); err != nil {
		return fmt.Errorf("failed reading response from first backend: %w", err)
	}

	resp := resp1.Bytes()

	// forward it to the client.
	if _, err := io.Copy(conn, bytes.NewReader(resp)); err != nil {
		return fmt.Errorf("failed writing response to client: %w", err)
	}

	log.WithField("resp", fmt.Sprintf("%X", resp)).Info("replied")
	_ = firstConn.Close()

	// send the request to the second backend.
	secondConn, err := net.Dial("unix", cli.SecondRemoteAddr)
	if err != nil {
		return fmt.Errorf("unable to connect to second backend: %w", err)
	}
	defer secondConn.Close()

	// send the request to the second backend.
	if _, err := io.Copy(secondConn, bytes.NewReader(rq)); err != nil {
		return fmt.Errorf("unable to send request to second backend: %w", err)
	}

	// read the response from the second backend.
	var resp2 bytes.Buffer
	if _, err := io.Copy(&resp2, secondConn); err != nil {
		return fmt.Errorf("unable to read response from second backend: %w", err)
	}

	// compare the output from the first with the second and log an error if they mismatch.
	if !bytes.Equal(resp1.Bytes(), resp2.Bytes()) {
		log.WithField("resp1", fmt.Sprintf("%X", resp1.Bytes())).WithField("resp2", fmt.Sprintf("%X", resp2.Bytes())).Error("response mismatch")
	}

	return nil
}
