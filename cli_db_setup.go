//go:build !bindings || clionly

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"

	installer "cyberstab-installer/pkg/installer"
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
			if isWindows() {
				cliHint("На носителе найдены установщики PostgreSQL:")
			} else {
				osInfo, osErr := installer.DetectLinuxOS()
				if osErr == nil {
					cliHint(fmt.Sprintf("Доступные пакеты PostgreSQL в репозитории (%s):", osInfo.Type))
				} else {
					cliHint("Доступные пакеты PostgreSQL в репозитории:")
				}
			}
			opts := make([]huh.Option[string], len(installers))
			for i, inst := range installers {
				if isWindows() {
					opts[i] = huh.NewOption(fmt.Sprintf("%s — %s", inst.Label, inst.Path), inst.Path)
				} else {
					opts[i] = huh.NewOption(fmt.Sprintf("%s (apt/dnf)", inst.Label), inst.Path)
				}
			}
			choice, err := promptSelect("Выберите версию для установки", opts, installers[0].Path)
			if err != nil {
				return "", "", "", err
			}
			if isWindows() {
				cliHint("Запуск установщика СУБД… Дождитесь завершения.")
			} else {
				cliHint("Установка PostgreSQL из пакетов… Это может занять несколько минут.")
			}
			if err := app.InstallPostgresInstaller(choice); err != nil {
				cliError(err.Error())
			} else {
				if isWindows() {
					cliOK("Установщик СУБД завершён — проверяем…")
				} else {
					cliOK("Пакеты PostgreSQL установлены — проверяем…")
				}
			}
			continue
		}

		noPkgHint := "СУБД не найдена и пакеты в репозитории недоступны"
		if isWindows() {
			noPkgHint = "СУБД не найдена и установщик на USB отсутствует"
		}
		action, err := promptSelect(noPkgHint, []huh.Option[string]{
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
