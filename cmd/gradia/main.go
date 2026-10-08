package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/paths"
	"github.com/shesh/gradia/internal/storage/sqlite"
	"github.com/shesh/gradia/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка запуска: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dbFlag := flag.String("db", "", "путь к SQLite-базе (по умолчанию — каталог данных пользователя)")
	flag.Parse()
	path, err := paths.DB(*dbFlag)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	store, err := sqlite.Open(ctx, path)
	if err != nil {
		cancel()
		return err
	}
	defer store.Close()
	defer cancel()

	students := app.NewStudentService(store)
	_, err = tea.NewProgram(tui.NewApp(ctx, students), tea.WithContext(ctx)).Run()
	return err
}
