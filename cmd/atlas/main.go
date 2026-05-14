// Copyright 2021-present The Atlas Authors. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"ariga.io/atlas/cmd/atlas/internal/cmdapi"
	_ "ariga.io/atlas/cmd/atlas/internal/docker"
	_ "ariga.io/atlas/sql/mysql"
	_ "ariga.io/atlas/sql/mysql/mysqlcheck"
	_ "ariga.io/atlas/sql/postgres"
	_ "ariga.io/atlas/sql/postgres/postgrescheck"
	_ "ariga.io/atlas/sql/sqlite"
	_ "ariga.io/atlas/sql/sqlite/sqlitecheck"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

func main() {
	cmdapi.Root.SetOut(os.Stdout)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// On first signal seen, cancel the context. On the second signal, force stop immediately.
		stop := make(chan os.Signal, 2)
		defer close(stop)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		defer signal.Stop(stop)
		<-stop   // wait for first interrupt
		cancel() // cancel context to gracefully stop
		fmt.Fprintln(cmdapi.Root.OutOrStdout(), "\ninterrupt received, wait for exit or ^C to terminate")
		// Wait for the context to be canceled. Issuing a second interrupt will cause the process to force stop.
		<-stop // will not block if no signal received due to main routine exiting
		os.Exit(1)
	}()
	ctx, err := extendContext(ctx)
	cobra.CheckErr(err)
	err = cmdapi.Root.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}

func extendContext(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
