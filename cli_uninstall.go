//go:build cyberstab_uninstaller && (!bindings || clionly)

package main

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

func runCLIUninstall(app *App) error {
	if err := requireTTY(); err != nil {
		return err
	}

	printCLIBanner("Удаление", "консольный режим")
	cliHint("Будут удалены папка установки, база okidoci_db, роли okidoci_*/oki_*,")
	cliHint("задачи автозапуска и ярлыки. Сам PostgreSQL не удаляется.")

	installDir := defaultInstallDir()
	if isWindows() {
		v, err := promptPathInputOrFallback("Папка установки", installDir, installDir)
		if err != nil {
			return err
		}
		if v != "" {
			installDir = v
		}
	} else {
		printCLISection("Папка установки")
		cliSummaryLine("Путь", installDir)
	}

	skipDB, err := promptConfirm("Пропустить удаление базы данных?", false)
	if err != nil {
		return err
	}

	dbEngine := ""
	pgUser := "postgres"
	pgPass := ""

	if !skipDB {
		printCLISection("PostgreSQL / СУБД")
		result, err := app.CheckDbInstalled()
		if err != nil {
			return err
		}
		if !result.Installed {
			cliWarn("СУБД не найдена — очистка БД будет пропущена.")
			skipDB = true
		} else {
			engines := result.Engines
			if len(engines) > 1 {
				opts := make([]huh.Option[string], len(engines))
				for i, e := range engines {
					opts[i] = huh.NewOption(fmt.Sprintf("%s (%s)", e.Label, e.BinDir), e.Kind)
				}
				kind, err := promptSelect("Выберите движок СУБД", opts, engines[0].Kind)
				if err != nil {
					return err
				}
				dbEngine = kind
			} else if len(engines) == 1 {
				dbEngine = engines[0].Kind
			}
			pgUser, pgPass, err = promptPostgresCredentialsUninstall(app)
			if err != nil {
				if err.Error() == "skip_db" {
					skipDB = true
				} else {
					return err
				}
			}
		}
	}

	printCLISection("Подтверждение")
	cliSummaryLine("Папка", installDir)
	if skipDB {
		cliSummaryLine("БД", "пропуск")
	} else {
		cliSummaryLine("БД", "удалить okidoci_db и роли")
	}

	ok, err := promptConfirm("Подтвердить удаление?", false)
	if err != nil {
		return err
	}
	if !ok {
		cliHint("Отменено.")
		return nil
	}

	fmt.Println()
	printCLISection("Удаление")

	opts := UninstallOptions{
		InstallDir:       installDir,
		DBEngine:         dbEngine,
		PostgresUser:     pgUser,
		PostgresPassword: pgPass,
		SkipDB:           skipDB,
	}

	res, err := app.runUninstallCore(opts)
	if err != nil {
		return err
	}
	fmt.Println()
	cliOK("Готово.")
	cliSummaryLine("Результат", res.Report)
	if res.Deferred {
		cliWarn("Некоторые файлы будут удалены после завершения процесса.")
	}
	return nil
}
