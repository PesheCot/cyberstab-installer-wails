package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

func requireTTY() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("консольный режим требует интерактивный терминал (запустите из SSH/tty, не через pipe)")
	}
	return nil
}

func readLine(prompt string) (string, error) {
	if prompt != "" {
		fmt.Print(prompt)
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func askYesNo(prompt string, defaultYes bool) (bool, error) {
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}
	for {
		ans, err := readLine(fmt.Sprintf("%s %s: ", prompt, hint))
		if err != nil {
			return false, err
		}
		if ans == "" {
			return defaultYes, nil
		}
		switch strings.ToLower(ans) {
		case "y", "yes", "д", "да":
			return true, nil
		case "n", "no", "н", "нет":
			return false, nil
		default:
			fmt.Println("Введите y или n.")
		}
	}
}

func askChoice(prompt string, options []string, defaultIdx int) (int, error) {
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	for {
		ans, err := readLine(fmt.Sprintf("%s [%d]: ", prompt, defaultIdx+1))
		if err != nil {
			return 0, err
		}
		if ans == "" {
			return defaultIdx, nil
		}
		var n int
		if _, err := fmt.Sscanf(ans, "%d", &n); err != nil || n < 1 || n > len(options) {
			fmt.Println("Неверный выбор.")
			continue
		}
		return n - 1, nil
	}
}
