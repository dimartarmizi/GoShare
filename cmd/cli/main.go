package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"goshare/app"
	"goshare/app/network"
)

func main() {
	mode := flag.String("mode", "server", "mode: server | discover | send")
	listen := flag.String("listen", ":9000", "TCP listen address")
	saveDir := flag.String("save-dir", "./received", "directory for incoming files")
	name := flag.String("name", "", "device name for discovery")
	discoveryPort := flag.Int("discovery-port", 9999, "UDP discovery port")
	target := flag.String("target", "", "target address for send mode (e.g., 192.168.1.10:9000)")
	filePath := flag.String("file", "", "file path for send mode")
	timeout := flag.Duration("timeout", 3*time.Second, "discovery timeout")
	flag.Parse()

	tcpPort, err := parsePort(*listen)
	if err != nil {
		log.Fatal(err)
	}

	cfg := app.Config{
		ListenAddr:    *listen,
		SaveDir:       *saveDir,
		DeviceName:    *name,
		DeviceTCPPort: tcpPort,
		DiscoveryPort: *discoveryPort,
		ChunkSize:     1024 * 1024,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	a := app.New(cfg)
	a.OnFileReceived = func(meta network.FileMetadata, path string, err error) {
		if err != nil {
			log.Printf("receive failed (%s): %v", meta.FileName, err)
			return
		}
		log.Printf("received file: %s -> %s (%d bytes)", meta.FileName, path, meta.Size)
	}

	switch *mode {
	case "server":
		if err := a.Start(ctx); err != nil {
			log.Fatal(err)
		}
		log.Printf("server ready on %s, saving files to %s", *listen, *saveDir)
		<-ctx.Done()
		log.Printf("shutting down")

	case "discover":
		devices, err := a.DiscoverDevices(ctx, *timeout)
		if err != nil {
			log.Fatal(err)
		}
		if len(devices) == 0 {
			fmt.Println("no devices found")
			return
		}
		for _, d := range devices {
			fmt.Printf("- %s (%s) %s:%d\n", d.Name, d.ID, d.IP, d.Port)
		}

	case "send":
		if *target == "" || *filePath == "" {
			log.Fatal("send mode requires -target and -file")
		}
		abs, err := filepath.Abs(*filePath)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := os.Stat(abs); err != nil {
			log.Fatal(err)
		}
		log.Printf("sending %s -> %s", abs, *target)
		err = a.Connections.SendFile(ctx, *target, abs, func(done, total int64) {
			pct := float64(done) / float64(total) * 100
			log.Printf("progress: %.1f%% (%d/%d)", pct, done, total)
		})
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("send complete")

	default:
		log.Fatalf("unknown mode: %s", *mode)
	}
}

func parsePort(listenAddr string) (int, error) {
	_, portStr, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return 0, fmt.Errorf("listen address must be host:port, got %q", listenAddr)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return 0, fmt.Errorf("listen address must end with a positive numeric port, got %q", listenAddr)
	}
	return port, nil
}
