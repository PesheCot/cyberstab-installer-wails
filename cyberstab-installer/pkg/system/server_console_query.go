package system

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

type ServerSession struct {
	UserID   int    `json:"userId"`
	Login    string `json:"login"`
	Username string `json:"username"`
	IP       string `json:"ip"`
	Company  string `json:"company"`
	Module   string `json:"module"`
}

type ServerLiveInfo struct {
	Sessions []ServerSession
}

func (i ServerLiveInfo) SessionCount() int {
	return len(i.Sessions)
}

var (
	liveInfoMu    sync.Mutex
	liveInfoCache struct {
		installDir string
		at         time.Time
		info       ServerLiveInfo
		err        error
	}
	liveInfoTTL = 5 * time.Second
)

func InvalidateServerLiveInfoCache() {
	liveInfoMu.Lock()
	defer liveInfoMu.Unlock()
	liveInfoCache.at = time.Time{}
}

func findServerConsoleExe(installDir string) (string, string, error) {
	base := serverBundleDir(installDir)
	workDir := filepath.Join(base, "serverconsole")
	var names []string
	if runtime.GOOS == "windows" {
		names = []string{"CyberstabServerConsoleWindows.exe", "serverconsole.exe"}
	} else {
		names = []string{"CyberstabServerConsoleLinux", "serverconsole"}
	}
	for _, name := range names {
		exe := filepath.Join(workDir, name)
		if st, err := os.Stat(exe); err == nil && !st.IsDir() {
			return exe, workDir, nil
		}
	}
	return "", workDir, fmt.Errorf("консоль сервера не найдена в %s", workDir)
}

func decodeConsoleOutput(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	candidates := []string{string(raw)}
	if runtime.GOOS == "windows" {
		if b, err := charmap.CodePage866.NewDecoder().Bytes(raw); err == nil {
			candidates = append(candidates, string(b))
		}
		if b, err := charmap.Windows1251.NewDecoder().Bytes(raw); err == nil {
			candidates = append(candidates, string(b))
		}
	}
	return bestConsoleText(candidates)
}

func bestConsoleText(candidates []string) string {
	best := candidates[0]
	bestScore := scoreConsoleText(best)
	for _, candidate := range candidates[1:] {
		if score := scoreConsoleText(candidate); score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return best
}

func scoreConsoleText(s string) int {
	if looksLikeMojibake(s) {
		return -100
	}
	score := 0
	if utf8.ValidString(s) {
		score += 3
	}
	for _, r := range s {
		switch {
		case r == utf8.RuneError:
			score -= 8
		case r >= 0x0400 && r <= 0x04FF:
			score += 4
		case unicode.Is(unicode.Cyrillic, r):
			score += 4
		case r == '€' || r == '√' || r == '—':
			score -= 4
		case r >= 0x0080 && r <= 0x024F:
			score -= 2
		}
	}
	return score
}

func looksLikeMojibake(s string) bool {
	if strings.Contains(s, "Рђ") || strings.Contains(s, "РЎ") || strings.Contains(s, "Рџ") {
		return true
	}
	if strings.ContainsAny(s, "€√—") {
		return true
	}
	return false
}

func runServerConsole(installDir, stdin string) (string, error) {
	return runServerConsoleWithTimeout(installDir, stdin, 45*time.Second)
}

func runServerConsoleWithTimeout(installDir, stdin string, timeout time.Duration) (string, error) {
	ports := LoadServerPorts(installDir)
	if isLocalTCPPortOpen(ports.NetworkPort, 400*time.Millisecond) {
		if !isLocalTCPPortOpen(ports.ManagementPort, 400*time.Millisecond) {
			return "", fmt.Errorf("management port closed")
		}
	}
	exe, workDir, err := findServerConsoleExe(installDir)
	if err != nil {
		return "", err
	}

	var cmd *exec.Cmd
	if timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd = exec.CommandContext(ctx, exe)
	} else {
		cmd = exec.Command(exe)
	}
	cmd.Dir = workDir
	hideCmd(cmd)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(decodeConsoleOutput(stderr.Bytes()))
		if msg == "" {
			msg = err.Error()
		}
		return decodeConsoleOutput(stdout.Bytes()), fmt.Errorf("%s", msg)
	}
	return decodeConsoleOutput(stdout.Bytes()), nil
}

// QueryServerLiveInfo asks the server console (RMI) for active sessions.
func QueryServerLiveInfo(installDir string) (ServerLiveInfo, error) {
	installDir = installDirOrDefault(installDir)
	liveInfoMu.Lock()
	defer liveInfoMu.Unlock()
	if liveInfoCache.installDir == installDir && time.Since(liveInfoCache.at) < liveInfoTTL {
		return liveInfoCache.info, liveInfoCache.err
	}
	info, err := queryServerLiveInfoNow(installDir)
	liveInfoCache.installDir = installDir
	liveInfoCache.at = time.Now()
	liveInfoCache.info = info
	liveInfoCache.err = err
	return info, err
}

func queryServerLiveInfoNow(installDir string) (ServerLiveInfo, error) {
	out, err := runServerConsole(installDir, "connections\nquit\n")
	if err != nil {
		return ServerLiveInfo{}, err
	}
	return parseServerConsoleOutput(out), nil
}

func parseConnectionRow(line string) (ServerSession, bool) {
	line = strings.TrimRight(line, "\r")
	trimmed := strings.TrimSpace(strings.TrimPrefix(line, "> "))
	if trimmed == "" {
		return ServerSession{}, false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "id") && strings.Contains(lower, "login") {
		return ServerSession{}, false
	}
	parts := strings.Split(trimmed, "\t")
	if len(parts) < 6 {
		return ServerSession{}, false
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil || id <= 0 {
		return ServerSession{}, false
	}
	return ServerSession{
		UserID:   id,
		Login:    parts[1],
		Username: parts[2],
		IP:       parts[3],
		Company:  parts[4],
		Module:   parts[5],
	}, true
}

func parseServerConsoleOutput(raw string) ServerLiveInfo {
	info := ServerLiveInfo{}
	for _, line := range strings.Split(raw, "\n") {
		if session, ok := parseConnectionRow(line); ok {
			info.Sessions = append(info.Sessions, session)
		}
	}
	return info
}

func DisconnectServerUser(installDir string, userID int) error {
	if userID <= 0 {
		return fmt.Errorf("некорректный идентификатор пользователя")
	}
	InvalidateServerLiveInfoCache()
	_, err := runServerConsole(installDir, fmt.Sprintf("disconnect %d\nquit\n", userID))
	return err
}

func DisconnectAllServerUsers(installDir string) error {
	InvalidateServerLiveInfoCache()
	live, err := queryServerLiveInfoNow(installDir)
	if err != nil {
		return err
	}
	if len(live.Sessions) == 0 {
		return nil
	}
	var script strings.Builder
	for _, s := range live.Sessions {
		script.WriteString(fmt.Sprintf("disconnect %d\n", s.UserID))
	}
	script.WriteString("quit\n")
	InvalidateServerLiveInfoCache()
	_, err = runServerConsole(installDir, script.String())
	return err
}
