package installer

import (
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// dbupdater quiet-mode stdout protocol (ru.yarsec.lib.commonupdater.enums.EStateMessage).
const (
	dbUpdaterPrefixResult   = "[result]:"
	dbUpdaterPrefixState    = "[state]:"
	dbUpdaterPrefixError    = "[err]:"
	dbUpdaterPrefixRestored = "[restored]:"
	dbUpdaterPrefixVersion  = "[version]:"
)

func dbUpdaterOutputEncoding() encoding.Encoding {
	if runtime.GOOS == "windows" {
		return charmap.Windows1251
	}
	return unicode.UTF8
}

type dbUpdaterLineKind int

const (
	dbUpdaterLineUnknown dbUpdaterLineKind = iota
	dbUpdaterLineState
	dbUpdaterLineResult
	dbUpdaterLineError
	dbUpdaterLineRestored
	dbUpdaterLineVersion
)

type dbUpdaterLine struct {
	Kind    dbUpdaterLineKind
	Payload string
}

func parseDbUpdaterLine(line string) (dbUpdaterLine, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return dbUpdaterLine{}, false
	}
	// Match EStateMessage.getByMessage enum order.
	for _, item := range []struct {
		prefix string
		kind   dbUpdaterLineKind
	}{
		{dbUpdaterPrefixResult, dbUpdaterLineResult},
		{dbUpdaterPrefixState, dbUpdaterLineState},
		{dbUpdaterPrefixError, dbUpdaterLineError},
		{dbUpdaterPrefixRestored, dbUpdaterLineRestored},
		{dbUpdaterPrefixVersion, dbUpdaterLineVersion},
	} {
		if strings.HasPrefix(line, item.prefix) {
			return dbUpdaterLine{Kind: item.kind, Payload: strings.TrimPrefix(line, item.prefix)}, true
		}
	}
	return dbUpdaterLine{}, false
}

func dbUpdaterProgressPercent(statePayload string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(statePayload), "/")
	if len(parts) != 2 {
		return 0, false
	}
	processed, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	total, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || total <= 0 || processed < 0 {
		return 0, false
	}
	pct := int(processed * 100 / total)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct, true
}

func dbUpdaterFinishedSuccessfully(errors []string, result string, hasResult bool) bool {
	return len(errors) == 0 && hasResult && result != "-1"
}
