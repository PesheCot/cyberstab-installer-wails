//go:build !bindings

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
)

func promptDatabaseSetup(app *App, sourceRoot string) (dbEngine, pgUser, pgPass string, err error) {
	printCLISection("PostgreSQL / СУБД")

	for {
		result, err := app.CheckDbInstalled()
		if err != nil {
			return "", "", "", err
		}
		if result.Installed {
			return selectInstalledEngine(app, result)
		}

		cliWarn("СУБД не найдена на этом компьютере.")
		installers := app.ListPostgresInstallers(sourceRoot)

		if len(installers) > 0 {
			cliHint("На носителе найдены установщики PostgreSQL:")
			opts := make([]huh.Option[string], len(installers))
			for i, inst := range installers {
				opts[i] = huh.NewOption(fmt.Sprintf("%s — %s", inst.Label, inst.Path), inst.Path)
			}
			choice, err := promptSelect("Выберите версию для установки", opts, installers[0].Path)
			if err != nil {
				return "", "", "", err
			}
			cliHint("Запуск установщика СУБД… Дождитесь завершения.")
			if err := app.InstallPostgresInstaller(choice); err != nil {
				cliError(err.Error())
			} else {
				cliOK("Установщик СУБД завершён — проверяем…")
			}
			continue
		}

		action, err := promptSelect("СУБД не найдена и установщик на USB отсутствует", []huh.Option[string]{
			huh.NewOption("Указать папку с bin/psql вручную", "manual"),
			huh.NewOption("Повторить поиск", "retry"),
			huh.NewOption("Отменить установку", "cancel"),
		}, "manual")
		if err != nil {
			return "", "", "", err
		}
		switch action {
		case "cancel":
			return "", "", "", fmt.Errorf("установка отменена: нужна СУБД PostgreSQL")
		case "retry":
			continue
		case "manual":
			path, err := promptPathInputOrFallback("Путь к папке СУБД (должен содержать bin/psql)", "", "")
			if err != nil {
				return "", "", "", err
			}
			bin := filepath.Join(strings.TrimSpace(path), "bin")
			psql := filepath.Join(bin, "psql")
			if isWindows() {
				psql += ".exe"
			}
			if _, err := os.Stat(psql); err != nil {
				cliError(fmt.Sprintf("не найден %s", psql))
				continue
			}
			_ = app.SelectDbEngineBin(bin)
			result, err = app.CheckDbInstalled()
			if err != nil || !result.Installed {
				cliError("СУБД не распознана по указанному пути")
				continue
			}
			return selectInstalledEngine(app, result)
		}
	}
}

func selectInstalledEngine(app *App, result DbCheckResult) (dbEngine, pgUser, pgPass string, err error) {
	engines := result.Engines
	if len(engines) > 1 {
		opts := make([]huh.Option[string], len(engines))
		for i, e := range engines {
			opts[i] = huh.NewOption(fmt.Sprintf("%s (%s)", e.Label, e.BinDir), e.BinDir)
		}
		binDir, err := promptSelect("Выберите движок СУБД", opts, engines[0].BinDir)
		if err != nil {
			return "", "", "", err
		}
		_ = app.SelectDbEngineBin(binDir)
		for _, e := range engines {
			if e.BinDir == binDir {
				dbEngine = e.Kind
				break
			}
		}
	} else if len(engines) == 1 {
		dbEngine = engines[0].Kind
		cliSummaryLine("СУБД", engines[0].Label)
		_ = app.SelectDbEngineBin(engines[0].BinDir)
	}

	pgUser, pgPass, err = promptPostgresCredentials(app)
	return dbEngine, pgUser, pgPass, err
}
