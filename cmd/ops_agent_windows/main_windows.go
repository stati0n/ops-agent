package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kardianos/osext"
	"golang.org/x/sys/windows/svc"
)

const serviceName = "Google Cloud Ops Agent"

var (
	installServices = flag.Bool("install", false, "whether to install the services")
	dataDirectory   = flag.String("data-directory", filepath.Join(os.Getenv("PROGRAMDATA"), `Google/Cloud Operations/Ops Agent`), "where to store runtime files")
)

func main() {
	flag.Parse()
	if err := initServices(*dataDirectory); err != nil {
		log.Fatal(err)
	}
	if ok, err := svc.IsWindowsService(); ok && err == nil {
		if err := run(serviceName, *dataDirectory); err != nil {
			log.Fatal(err)
		}
	} else if err != nil {
		log.Fatalf("failed to talk to service control manager: %v", err)
	} else if *installServices {
		if err := install(); err != nil {
			log.Fatal(err)
		}
		log.Printf("installed services")
	} else {
		fmt.Println("Invoked as a standalone program with no flags. Nothing to do.")
	}
}

var services []struct {
	name    string
	exepath string
	args    []string
}

func initServices(dataDirectory string) error {
	// Identify relevant paths
	self, err := osext.Executable()
	if err != nil {
		return fmt.Errorf("could not determine own path: %w", err)
	}
	base, err := osext.ExecutableFolder()
	if err != nil {
		return fmt.Errorf("could not determine binary path: %w", err)
	}
	configOutDir := filepath.Join(dataDirectory, "generated_configs")
	if err := os.MkdirAll(configOutDir, 0644); err != nil {
		return err
	}
	fluentbitStoragePath := filepath.Join(dataDirectory, `run\buffers`)
	if err := os.MkdirAll(fluentbitStoragePath, 0644); err != nil {
		return err
	}
	logDirectory := filepath.Join(os.Getenv("PROGRAMDATA"), dataDirectory, "log")
	if err := os.MkdirAll(logDirectory, 0644); err != nil {
		return err
	}
	// TODO: Write meaningful descriptions for these services
	services = []struct {
		name    string
		exepath string
		args    []string
	}{
		{
			serviceName,
			self,
			[]string{"-in", filepath.Join(base, "../config/config.yaml"), "-out", configOutDir},
		},
		{
			fmt.Sprintf("%s - Metrics Agent", serviceName),
			filepath.Join(base, "google-cloud-metrics-agent_windows_amd64.exe"),
			[]string{
				"--add-instance-id=false",
				"--config=" + filepath.Join(configOutDir, `otel\otel.yaml`),
			},
		},
		{
			// TODO: fluent-bit hardcodes a service name of "fluent-bit"; do we need to match that?
			fmt.Sprintf("%s - Logging Agent", serviceName),
			filepath.Join(base, "fluent-bit.exe"),
			[]string{
				"-c", filepath.Join(configOutDir, `fluentbit\fluent_bit_main.conf`),
				"-R", filepath.Join(configOutDir, `fluentbit\fluent_bit_parser.conf`),
				"--storage_path", fluentbitStoragePath,
				"--log_file", filepath.Join(logDirectory, "logging-module.log"),
			},
		},
	}
	return nil
}
