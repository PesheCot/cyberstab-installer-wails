//go:build !bindings

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

func requireTTY() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("консольный режим требует интерактивный терминал (запустите из SSH/tty, не через pipe)")
	}
	return nil
}

func cliTheme() *huh.Theme {
	t := huh.ThemeBase()
	t.Focused.Title = t.Focused.Title.Foreground(lipgloss.Color("#F5C542"))
	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(lipgloss.Color("#7EC8E3")).SetString("›")
	t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(lipgloss.Color("#7EC8E3")).Bold(true)
	t.Focused.UnselectedOption = lipgloss.NewStyle().Foreground(lipgloss.Color("#C8D6E5"))
	t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#7EC8E3"))
	t.Focused.TextInput.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	t.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7A8C"))
	t.Blurred.Title = t.Blurred.Title.Foreground(lipgloss.Color("#9AA5B1"))
	return t
}

func runForm(fields ...huh.Field) error {
	form := huh.NewForm(huh.NewGroup(fields...)).WithTheme(cliTheme()).WithShowHelp(true)
	return form.Run()
}

func promptSelect[T comparable](title string, options []huh.Option[T], initial T) (T, error) {
	var value T
	value = initial
	if err := runForm(huh.NewSelect[T]().
		Title(title).
		Options(options...).
		Value(&value)); err != nil {
		return value, err
	}
	return value, nil
}

func promptConfirm(title string, defaultVal bool) (bool, error) {
	var value bool
	value = defaultVal
	if err := runForm(huh.NewConfirm().
		Title(title).
		Affirmative("Да").
		Negative("Нет").
		Value(&value)); err != nil {
		return false, err
	}
	return value, nil
}

func promptInput(title, placeholder, defaultVal string) (string, error) {
	var value string
	if defaultVal != "" {
		value = defaultVal
	}
	field := huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Value(&value)
	if err := runForm(field); err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" && defaultVal != "" {
		return defaultVal, nil
	}
	return value, nil
}

func promptPassword(title string) (string, error) {
	var value string
	if err := runForm(huh.NewInput().
		Title(title).
		EchoMode(huh.EchoModePassword).
		Value(&value)); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

type componentChoice string

const (
	compServer  componentChoice = "server"
	compDB      componentChoice = "db"
	compClient  componentChoice = "client"
)

func promptInstallComponents() (installServer, installClients, installDB bool, err error) {
	var primary componentChoice

	if err := runForm(
		huh.NewSelect[componentChoice]().
			Title("Что установить?").
			Description("Сервер и база данных — взаимоисключающие, как в графическом мастере").
			Options(
				huh.NewOption("Сервер (полная установка)", compServer),
				huh.NewOption("Только база данных (dbupdater)", compDB),
				huh.NewOption("Только клиент", compClient),
			).
			Value(&primary),
	); err != nil {
		return false, false, false, err
	}

	switch primary {
	case compServer:
		installServer = true
		installClients, err = promptConfirm("Дополнительно установить клиент?", true)
		if err != nil {
			return false, false, false, err
		}
	case compDB:
		installDB = true
		cliHint("Будут скопированы serverconsole и dbupdater для инициализации БД.")
		installClients, err = promptConfirm("Дополнительно установить клиент?", true)
		if err != nil {
			return false, false, false, err
		}
	case compClient:
		installClients = true
	default:
		return false, false, false, fmt.Errorf("нужно выбрать хотя бы один компонент")
	}

	return installServer, installClients, installDB, nil
}

const pgMaxAttempts = 3

func runPostgresPasswordReset(app *App, user string) (string, error) {
	user = strings.TrimSpace(user)
	if user == "" {
		return "", fmt.Errorf("укажите пользователя PostgreSQL")
	}

	printCLISection("Забыли пароль?")
	cliHint("Сброс через runuser -u postgres; при md5 в pg_hba.conf правила временно меняются на peer и восстанавливаются.")
	cliSummaryLine("Пользователь", user)

	newPass, err := promptPassword("Новый пароль")
	if err != nil {
		return "", err
	}
	if newPass == "" {
		return "", fmt.Errorf("новый пароль не должен быть пустым")
	}
	newPass2, err := promptPassword("Повторите новый пароль")
	if err != nil {
		return "", err
	}
	if newPass != newPass2 {
		return "", fmt.Errorf("пароли не совпадают")
	}

	if err := app.ResetPostgresPassword(user, newPass); err != nil {
		return "", err
	}
	cliOK(fmt.Sprintf("Пароль для %s изменён", user))

	if err := app.VerifyPostgresPassword(user, newPass); err != nil {
		return "", fmt.Errorf("пароль сменён, но проверка не прошла: %w", err)
	}
	cliOK("Подключение и права подтверждены")
	return newPass, nil
}

func promptPostgresAuthMode() (string, error) {
	return promptSelect("Учётные данные PostgreSQL", []huh.Option[string]{
		huh.NewOption("Ввести пароль", "enter"),
		huh.NewOption("Забыли пароль — задать новый", "reset"),
	}, "enter")
}

func pgRetryOptionsInstall() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Ввести другой пароль", "password"),
		huh.NewOption("Забыли пароль — задать новый", "reset"),
		huh.NewOption("Сменить пользователя и пароль", "both"),
		huh.NewOption("Отменить установку", "cancel"),
	}
}

func pgRetryOptionsUninstall() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Ввести другой пароль", "password"),
		huh.NewOption("Забыли пароль — задать новый", "reset"),
		huh.NewOption("Сменить пользователя и пароль", "both"),
		huh.NewOption("Пропустить очистку БД", "skip"),
	}
}

func promptPostgresCredentials(app *App) (user, pass string, err error) {
	user = "postgres"
	retryUser := true

	for attempt := 1; attempt <= pgMaxAttempts; attempt++ {
		if retryUser {
			user, err = promptInput("Пользователь PostgreSQL", "postgres", user)
			if err != nil {
				return "", "", err
			}
			if user == "" {
				user = "postgres"
			}

			mode, err := promptPostgresAuthMode()
			if err != nil {
				return "", "", err
			}
			if mode == "reset" {
				pass, err = runPostgresPasswordReset(app, user)
				if err != nil {
					cliError(err.Error())
					if attempt >= pgMaxAttempts {
						break
					}
					continue
				}
				return user, pass, nil
			}
		}

		pass, err = promptPassword("Пароль PostgreSQL")
		if err != nil {
			return "", "", err
		}
		if pass == "" {
			cliWarn("Пароль не может быть пустым")
			continue
		}

		if err := app.VerifyPostgresPassword(user, pass); err == nil {
			cliOK("Подключение и права подтверждены")
			return user, pass, nil
		}

		cliError(fmt.Sprintf("Попытка %d из %d: не удалось подключиться — проверьте имя пользователя и пароль", attempt, pgMaxAttempts))

		if attempt >= pgMaxAttempts {
			break
		}

		action, err := promptSelect("Что сделать?", pgRetryOptionsInstall(), "password")
		if err != nil {
			return "", "", err
		}
		switch action {
		case "cancel":
			return "", "", fmt.Errorf("установка отменена пользователем")
		case "reset":
			pass, err = runPostgresPasswordReset(app, user)
			if err != nil {
				cliError(err.Error())
				retryUser = false
				continue
			}
			return user, pass, nil
		case "both":
			retryUser = true
		default:
			retryUser = false
		}
	}

	return "", "", fmt.Errorf("не удалось подключиться к PostgreSQL после %d попыток", pgMaxAttempts)
}

func promptPostgresCredentialsUninstall(app *App) (user, pass string, err error) {
	user = "postgres"
	retryUser := true

	for attempt := 1; attempt <= pgMaxAttempts; attempt++ {
		if retryUser {
			user, err = promptInput("Пользователь PostgreSQL", "postgres", user)
			if err != nil {
				return "", "", err
			}
			if strings.TrimSpace(user) == "" {
				user = "postgres"
			}

			mode, err := promptPostgresAuthMode()
			if err != nil {
				return "", "", err
			}
			if mode == "reset" {
				pass, err = runPostgresPasswordReset(app, user)
				if err != nil {
					cliError(err.Error())
					if attempt >= pgMaxAttempts {
						break
					}
					continue
				}
				return user, pass, nil
			}
		}

		pass, err = promptPassword("Пароль PostgreSQL")
		if err != nil {
			return "", "", err
		}

		if pass == "" {
			cliWarn("Пароль пустой — очистка БД может не выполниться")
			return user, pass, nil
		}

		if err := app.VerifyPostgresPassword(user, pass); err == nil {
			cliOK("Подключение подтверждено")
			return user, pass, nil
		}

		cliError(fmt.Sprintf("Попытка %d из %d: неверный пользователь или пароль", attempt, pgMaxAttempts))

		if attempt >= pgMaxAttempts {
			break
		}

		action, err := promptSelect("Что сделать?", pgRetryOptionsUninstall(), "password")
		if err != nil {
			return "", "", err
		}
		switch action {
		case "skip":
			return "", "", fmt.Errorf("skip_db")
		case "reset":
			pass, err = runPostgresPasswordReset(app, user)
			if err != nil {
				cliError(err.Error())
				retryUser = false
				continue
			}
			return user, pass, nil
		case "both":
			retryUser = true
		default:
			retryUser = false
		}
	}

	return "", "", fmt.Errorf("не удалось подключиться к PostgreSQL после %d попыток", pgMaxAttempts)
}
