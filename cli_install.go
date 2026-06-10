//go:build !cyberstab_uninstaller && !cyberstab_manager && !bindings

package main

import (
	"fmt"
	"strings"
)

func runCLIInstall(app *App) error {
	if err := requireTTY(); err != nil {
		return err
	}

	fmt.Println("=== Установщик Киберстаб (консольный режим) ===")
	fmt.Println()

	installServer, err := askYesNo("Установить сервер?", true)
	if err != nil {
		return err
	}
	installClients, err := askYesNo("Установить клиент?", true)
	if err != nil {
		return err
	}
	installDB, err := askYesNo("Установить/инициализировать базу данных?", false)
	if err != nil {
		return err
	}
	if installDB {
		installServer = false
	}
	if !installServer && !installClients && !installDB {
		return fmt.Errorf("нужно выбрать хотя бы один компонент")
	}

	needsPG := installServer || installDB
	dbEngine := ""
	pgUser := "postgres"
	pgPass := ""

	if needsPG {
		fmt.Println("\n--- PostgreSQL / СУБД ---")
		result, err := app.CheckDbInstalled()
		if err != nil {
			return err
		}
		if !result.Installed {
			return fmt.Errorf("СУБД не найдена. Установите PostgreSQL и повторите, либо укажите путь к bin вручную (GUI)")
		}
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
		if dbEngine != "" {
			_ = app.SelectDbEngine(dbEngine)
		}

		pgUser, err = readLine("Пользователь PostgreSQL [postgres]: ")
		if err != nil {
			return err
		}
		if pgUser == "" {
			pgUser = "postgres"
		}
		pgPass, err = readPassword("Пароль PostgreSQL: ")
		if err != nil {
			return err
		}
		if pgPass == "" {
			return fmt.Errorf("нужен пароль СУБД")
		}
		if err := app.VerifyPostgresPassword(pgUser, pgPass); err != nil {
			return fmt.Errorf("проверка учётных данных: %w", err)
		}
		fmt.Println("Подключение и права подтверждены.")
	}

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
		fmt.Printf("\nПапка установки: %s\n", installDir)
	}

	fmt.Println("\n--- Дистрибутив ---")
	sourceRoot, err := app.AutoDetectSourceRoot(installServer || installDB, installClients)
	if err != nil {
		return err
	}
	if sourceRoot != "" {
		fmt.Printf("Найден дистрибутив: %s\n", sourceRoot)
		use, err := askYesNo("Использовать этот путь?", true)
		if err != nil {
			return err
		}
		if !use {
			sourceRoot = ""
		}
	}
	if sourceRoot == "" {
		sourceRoot, err = readLine("Путь к папке с CyberstabServer*/CyberstabClient*: ")
		if err != nil {
			return err
		}
		if strings.TrimSpace(sourceRoot) == "" {
			return fmt.Errorf("укажите путь к дистрибутиву")
		}
	}

	dbAction := ""
	restoreSQL := ""
	if needsPG {
		fmt.Println("\n--- База данных ---")
		choices := []string{"Создать новую / пересоздать", "Оставить текущую", "Восстановить из .sql"}
		idx, err := askChoice("Действие с БД", choices, 0)
		if err != nil {
			return err
		}
		switch idx {
		case 0:
			dbAction = "new"
		case 1:
			dbAction = "skip"
		case 2:
			dbAction = "restore"
			restoreSQL, err = readLine("Путь к файлу .sql: ")
			if err != nil {
				return err
			}
			if strings.TrimSpace(restoreSQL) == "" {
				return fmt.Errorf("укажите путь к .sql")
			}
		}
	}

	reinstallExisting := true
	conflict, err := app.CheckInstallDirConflict(installDir)
	if err != nil {
		return err
	}
	if conflict.Exists {
		reinstall, err := askYesNo("Папка установки уже существует. Переустановить (скопировать файлы заново)?", true)
		if err != nil {
			return err
		}
		reinstallExisting = reinstall
	}

	fmt.Println("\n--- Сводка ---")
	fmt.Printf("  Сервер: %v, Клиент: %v, БД: %v\n", installServer, installClients, installDB)
	fmt.Printf("  Установка в: %s\n", installDir)
	fmt.Printf("  Дистрибутив: %s\n", sourceRoot)
	if needsPG {
		fmt.Printf("  СУБД: %s, пользователь: %s\n", dbEngine, pgUser)
	}
	ok, err := askYesNo("\nНачать установку?", true)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("Отменено.")
		return nil
	}

	opts := StartInstallOptions{
		InstallServer:     installServer,
		InstallClients:    installClients,
		InstallDB:         installDB,
		SourceRoot:        sourceRoot,
		DBEngine:          dbEngine,
		PostgresUser:      pgUser,
		PostgresPassword:  pgPass,
		InstallDir:        installDir,
		DbAction:            dbAction,
		RestoreSqlPath:    restoreSQL,
		ReinstallExisting: reinstallExisting,
	}

	var lastStep int
	cb := InstallCallbacks{
		OnSteps: func(steps []installerStepDTO) {
			for _, s := range steps {
				if s.IsActive && s.Index != lastStep {
					lastStep = s.Index
					fmt.Printf("[%d] %s\n", s.Index, s.Description)
				}
				if s.ErrorText != "" {
					fmt.Printf("  Ошибка: %s\n", s.ErrorText)
				}
			}
		},
		OnProgress: func(pct int, status string, _ []installerStepDTO) {
			fmt.Printf("[DB] %d%% %s\n", pct, status)
		},
		OnDeploy: func(pct int, status string) {
			fmt.Printf("[DEPLOY] %d%% %s\n", pct, status)
		},
	}

	if err := app.runInstall(opts, cb); err != nil {
		return err
	}
	fmt.Println("\nУстановка завершена успешно.")
	return nil
}
