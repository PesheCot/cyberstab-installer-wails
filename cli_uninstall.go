//go:build cyberstab_uninstaller && !bindings

package main

import (
	"fmt"
	"strings"
)

func runCLIUninstall(app *App) error {
	if err := requireTTY(); err != nil {
		return err
	}

	fmt.Println("=== Удаление Киберстаб (консольный режим) ===")
	fmt.Println()
	fmt.Println("Будут удалены папка установки, база okidoci_db, роли okidoci_*/oki_*,")
	fmt.Println("задачи автозапуска и ярлыки. Сам PostgreSQL не удаляется.")
	fmt.Println()

	installDir := defaultInstallDir()
	if isWindows() {
		v, err := readLine(fmt.Sprintf("Папка установки [%s]: ", installDir))
		if err != nil {
			return err
		}
		if v != "" {
			installDir = v
		}
	} else {
		fmt.Printf("Папка установки: %s\n", installDir)
	}

	skipDB, err := askYesNo("Пропустить удаление базы данных?", false)
	if err != nil {
		return err
	}

	dbEngine := ""
	pgUser := "postgres"
	pgPass := ""

	if !skipDB {
		result, err := app.CheckDbInstalled()
		if err != nil {
			return err
		}
		if !result.Installed {
			fmt.Println("Предупреждение: СУБД не найдена, очистка БД будет пропущена.")
			skipDB = true
		} else {
			engines := result.Engines
			if len(engines) > 1 {
				labels := make([]string, len(engines))
				for i, e := range engines {
					labels[i] = fmt.Sprintf("%s (%s)", e.Label, e.BinDir)
				}
				idx, err := askChoice("Выберите движок СУБД", labels, 0)
				if err != nil {
					return err
				}
				dbEngine = engines[idx].Kind
			} else if len(engines) == 1 {
				dbEngine = engines[0].Kind
			}
			pgUser, err = readLine("Пользователь PostgreSQL [postgres]: ")
			if err != nil {
				return err
			}
			if strings.TrimSpace(pgUser) == "" {
				pgUser = "postgres"
			}
			pgPass, err = readPassword("Пароль PostgreSQL: ")
			if err != nil {
				return err
			}
		}
	}

	ok, err := askYesNo("\nПодтвердить удаление?", false)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("Отменено.")
		return nil
	}

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
	fmt.Println("\nРезультат:", res.Report)
	if res.Deferred {
		fmt.Println("Некоторые файлы будут удалены после завершения процесса.")
	}
	return nil
}
