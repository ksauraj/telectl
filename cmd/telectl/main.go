// Command telectl runs a Telegram bot for managing Kubernetes clusters.
//
// telectl speaks to the Kubernetes API server directly through client-go: it
// never shells out to kubectl, and the container image does not ship one.
// Cluster access comes from a kubeconfig, so telectl inherits exactly the RBAC
// of whichever context it is pointed at.
//
// Usage:
//
//	telectl --token <telegram-token> --allowed-users <id[,id...]>
//	telectl --config /etc/telectl/telectl.yaml
//	telectl contexts     # list kubeconfig contexts and exit
//	telectl config       # print effective configuration and exit
//	telectl version      # print version and exit
//
// See docs/CONFIGURATION.md for the full precedence rules and
// docs/MENU_GUIDE.md for the in-chat interface.
//
// # Copyright 2024 Sauraj Kumar Singh
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ksauraj/telectl/cmd/telectl/cmd"
	"go.uber.org/zap"
)

// Build metadata, injected with -ldflags at build time. See the Makefile.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// A bootstrap logger, used only until the configured one replaces it: the
	// log level lives in the config, which cannot be read without a logger to
	// report failures through.
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	// Sync can fail when stderr is a terminal, and there is nothing useful to
	// do about it at process exit.
	defer func() { _ = logger.Sync() }()

	cmd.SetBuildInfo(version, commit, date)

	// Cancel the root context on SIGINT/SIGTERM so in-flight API calls and the
	// Telegram long poll unwind instead of being killed mid-request.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cmd.Execute(ctx); err != nil {
		logger.Fatal("Failed to execute command", zap.Error(err))
	}
}
