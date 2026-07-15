//go:build !cyberstab_uninstaller && !cyberstab_manager && (!bindings || clionly)

package main

import (
	"fmt"

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

	printCLISection("Лицензионное соглашение")
	cliHint("Лицензионный договор-оферта на программный комплекс «Киберстаб».")
	agreementPath, err := materializeUserAgreementFile()
	if err != nil {
		return fmt.Errorf("не удалось подготовить текст соглашения: %w", err)
	}
	cliSummaryLine("Полный текст", agreementPath)
	accepted, err := promptConfirm("Принимаете условия лицензионного соглашения?", false)
	if err != nil {
		return err
	}
	if !accepted {
		cliHint("Установка отменена: условия лицензии не приняты.")
		return nil
	}

	needsPG := installServer || installDB
	wantServerOrDB := installServer || installDB

	sourceRoot, err := promptDistroSourceRoot(app, wantServerOrDB, installClients)
	if err != nil {
		return err
	}

	dbEngine := ""
	pgUser := "postgres"
	pgPass := ""

	if needsPG {
		dbEngine, pgUser, pgPass, err = promptDatabaseSetup(app, sourceRoot)
		if err != nil {
			return err
		}
	}

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
			for {
				restoreSQL, err = promptPathInputOrFallback("Путь к файлу .sql", "", "")
				if err != nil {
					return err
				}
				if err := app.ValidateSqlBackupPath(restoreSQL); err != nil {
					cliError(err.Error())
					retry, rerr := promptConfirm("Указать другой файл?", true)
					if rerr != nil {
						return rerr
					}
					if !retry {
						return err
					}
					continue
				}
				break
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
