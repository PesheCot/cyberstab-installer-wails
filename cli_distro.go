//go:build !bindings

package main

import (
	"fmt"
	"strings"
)

func promptDistroSourceRoot(app *App, wantServerOrDB, wantClients bool) (string, error) {
	printCLISection("Дистрибутив")
	cliHint("Поиск на USB-накопителях…")

	sourceRoot, err := app.AutoDetectSourceRoot(wantServerOrDB, wantClients)
	if err != nil {
		return "", err
	}
	if sourceRoot != "" {
		if valid, err := validateDistroPath(app, sourceRoot, wantServerOrDB, wantClients); err != nil {
			return "", err
		} else if valid {
			cliSummaryLine("Найден на USB", sourceRoot)
			use, err := promptConfirm("Использовать этот путь?", true)
			if err != nil {
				return "", err
			}
			if use {
				return sourceRoot, nil
			}
		} else {
			cliWarn("На найденном носителе нет нужных папок дистрибутива.")
			sourceRoot = ""
		}
	} else {
		cliWarn("Не удалось найти дистрибутив на подключённых USB-накопителях.")
	}

	for {
		path, err := promptPathInputOrFallback(
			"Введите путь к папке с CyberstabServer*/CyberstabClient*",
			"",
			sourceRoot,
		)
		if err != nil {
			return "", err
		}
		path = strings.TrimSpace(path)
		if path == "" {
			return "", fmt.Errorf("укажите путь к дистрибутиву")
		}
		valid, err := validateDistroPath(app, path, wantServerOrDB, wantClients)
		if err != nil {
			return "", err
		}
		if valid {
			return path, nil
		}
		retry, err := promptConfirm("Попробовать другой путь?", true)
		if err != nil {
			return "", err
		}
		if !retry {
			return "", fmt.Errorf("установка отменена: не найден дистрибутив")
		}
		sourceRoot = ""
	}
}

func validateDistroPath(app *App, root string, wantServerOrDB, wantClients bool) (bool, error) {
	result, err := app.ValidateSourceRoot(root, wantServerOrDB, wantClients)
	if err != nil {
		cliError(err.Error())
		return false, nil
	}
	if result.Valid {
		return true, nil
	}
	cliError(fmt.Sprintf("В папке %s не найдено: %s", root, strings.Join(result.Missing, ", ")))
	return false, nil
}
