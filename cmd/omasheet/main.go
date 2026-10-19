package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"omasheet/internal/app"
)

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: shtui [workbook.csv|workbook.xlsx|workbook.xlsm]")
		os.Exit(2)
	}
	path := ""
	if len(os.Args) == 2 {
		path = os.Args[1]
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := app.Run(ctx, path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
