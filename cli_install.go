//go:build !cyberstab_uninstaller && !cyberstab_manager && !bindings

package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

func runCLIInstall(app *App) error {
	if err := requireTTY(); err != nil {
		return err
	}

	printCLIBanner("Установщик", "консольный режим")

	installServer, installClients, installDB, err := promptInstallComponents()
	if err != nil {
		return err
	}
	cliSummaryLine("Компоненты", componentModeLabel(installServer, installClients, installDB))

	needsPG := installServer || installDB
	dbEngine := ""
	pgUser := "postgres"
	pgPass := ""

	if needsPG {
		printCLISection("PostgreSQL / СУБД")
		result, err := app.CheckDbInstalled()
		if err != nil {
			return err
		}
		if !result.Installed {
			return fmt.Errorf("СУБД не найдена — установите PostgreSQL и повторите")
		}
		engines := result.Engines
		if len(engines) > 1 {
			opts := make([]huh.Option[string], len(engines))
			for i, e := range engines {
				opts[i] = huh.NewOption(fmt.Sprintf("%s (%s)", e.Label, e.BinDir), e.BinDir)
			}
			binDir, err := promptSelect("Выберите движок СУБД", opts, engines[0].BinDir)
			if err != nil {
				return err
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
		if err != nil {
			return err
		}
	}

	installDir := defaultInstallDir()
	if isWindows() {
		v, err := promptInput("Папка установки", installDir, installDir)
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

	printCLISection("Дистрибутив")
	sourceRoot, err := app.AutoDetectSourceRoot(installServer || installDB, installClients)
	if err != nil {
		return err
	}
	if sourceRoot != "" {
		cliSummaryLine("Найден", sourceRoot)
		use, err := promptConfirm("Использовать этот путь?", true)
		if err != nil {
			return err
		}
		if !use {
			sourceRoot = ""
		}
	}
	if sourceRoot == "" {
		sourceRoot, err = promptInput("Путь к папке с CyberstabServer*/CyberstabClient*", "", "")
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
		printCLISection("База данных")
		action, err := promptSelect("Действие с БД", []huh.Option[string]{
			huh.NewOption("Создать новую / пересоздать", "new"),
			huh.NewOption("Оставить текущую", "skip"),
			huh.NewOption("Восстановить из .sql", "restore"),
		}, "new")
		if err != nil {
			return err
		}
		dbAction = action
		if action == "restore" {
			restoreSQL, err = promptInput("Путь к файлу .sql", "", "")
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
		reinstall, err := promptConfirm("Папка уже существует — переустановить (скопировать заново)?", true)
		if err != nil {
			return err
		}
		reinstallExisting = reinstall
	}

	printCLISection("Сводка")
	cliSummaryLine("Компоненты", componentModeLabel(installServer, installClients, installDB))
	cliSummaryLine("Установка в", installDir)
	cliSummaryLine("Дистрибутив", sourceRoot)
	if needsPG {
		cliSummaryLine("СУБД", dbEngine)
		cliSummaryLine("Пользователь", pgUser)
	}

	ok, err := promptConfirm("Начать установку?", true)
	if err != nil {
		return err
	}
	if !ok {
		cliHint("Отменено.")
		return nil
	}

	fmt.Println()
	printCLISection("Установка")

	opts := StartInstallOptions{
		InstallServer:     installServer,
		InstallClients:    installClients,
		InstallDB:         installDB,
		SourceRoot:        sourceRoot,
		DBEngine:          dbEngine,
		PostgresUser:      pgUser,
		PostgresPassword:  pgPass,
		InstallDir:        installDir,
		DbAction:          dbAction,
		RestoreSqlPath:    restoreSQL,
		ReinstallExisting: reinstallExisting,
	}

	var lastStep int
	var lastDeployPct, lastDBPct int
	cb := InstallCallbacks{
		OnSteps: func(steps []installerStepDTO) {
			for _, s := range steps {
				if s.IsActive && s.Index != lastStep {
					lastStep = s.Index
					fmt.Println(cliLabelStyle.Render(fmt.Sprintf("  [%d]", s.Index)) + " " + s.Description)
				}
				if s.ErrorText != "" {
					cliError(s.ErrorText)
				}
			}
		},
		OnProgress: func(pct int, status string, _ []installerStepDTO) {
			if pct != lastDBPct {
				lastDBPct = pct
				cliProgressBar(pct, "БД: "+status)
			}
		},
		OnDeploy: func(pct int, status string) {
			if pct != lastDeployPct {
				lastDeployPct = pct
				cliProgressBar(pct, status)
			}
		},
	}

	if err := app.runInstall(opts, cb); err != nil {
		return err
	}
	fmt.Println()
	cliOK("Установка завершена успешно.")
	return nil
}
