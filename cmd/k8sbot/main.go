// k8s-telegram-bot - A Telegram bot for Kubernetes cluster management
//
// Copyright 2024 Sauraj Kumar Singh
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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

	"github.com/ksauraj/k8s-telegram-bot/cmd/k8sbot/cmd"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Create context that listens for interrupt signals
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Execute root command
	if err := cmd.Execute(ctx, logger); err != nil {
		logger.Fatal("Failed to execute command", zap.Error(err))
	}
}