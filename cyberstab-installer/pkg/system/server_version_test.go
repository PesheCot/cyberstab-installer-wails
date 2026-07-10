package system

import (
	"strings"
	"testing"
)

func TestReadVersionFromSignature(t *testing.T) {
	raw := "1;abc;installed_files;1;def;0:3.9.0"
	if m := signatureVersionRe.FindStringSubmatch(raw); len(m) != 2 || m[1] != "3.9.0" {
		t.Fatalf("signature parse: %#v", m)
	}
}

func TestParseServerConsoleOutput(t *testing.T) {
	raw := "welcome\n> ID\tLogin\tUsername\tIP\tCompany\tModule\n1\tadmin\tadmin\t/127.0.0.1\t1\tАдминистрирование\n> \n"
	info := parseServerConsoleOutput(raw)
	if info.SessionCount() != 1 {
		t.Fatalf("sessions: %d", info.SessionCount())
	}
	if info.Sessions[0].Login != "admin" {
		t.Fatalf("login: %q", info.Sessions[0].Login)
	}
	if info.Sessions[0].Module != "Администрирование" {
		t.Fatalf("module: %q", info.Sessions[0].Module)
	}
}

func TestParseServerConsoleOutputEmptySessions(t *testing.T) {
	raw := "> ID\tLogin\n> \n"
	info := parseServerConsoleOutput(raw)
	if info.SessionCount() != 0 {
		t.Fatalf("sessions: %d", info.SessionCount())
	}
}

func TestParseConnectionRowSkipsHeader(t *testing.T) {
	_, ok := parseConnectionRow("> ID\tLogin\tUsername\tIP\tCompany\tModule")
	if ok {
		t.Fatal("header should be skipped")
	}
}

func TestDecodeConsoleOutputPrefersUTF8Cyrillic(t *testing.T) {
	utf8Raw := append([]byte("1\tadmin\tadmin\t/127.0.0.1\t1\t"),
		[]byte{0xD0, 0x90, 0xD0, 0xB4, 0xD0, 0xBC, 0xD0, 0xB8, 0xD0, 0xBD, 0xD0, 0xB8, 0xD1, 0x81, 0xD1, 0x82, 0xD1, 0x80, 0xD0, 0xB8, 0xD1, 0x80, 0xD0, 0xBE, 0xD0, 0xB2, 0xD0, 0xB0, 0xD0, 0xBD, 0xD0, 0xB8, 0xD0, 0xB5, 0x20, 0xD0, 0x91, 0xD0, 0xA1, '\n'}...)
	got := decodeConsoleOutput(utf8Raw)
	if !strings.Contains(got, string([]byte{0xD0, 0x94})) { // Д from valid UTF-8 path
		if !strings.Contains(got, string([]byte{0xD0, 0x90})) { // А
			t.Fatalf("utf8 decode: %q", got)
		}
	}
}

func TestDecodeConsoleOutputCP866(t *testing.T) {
	raw := []byte{0x31, 0x09, 0xE0, 0xA4, 0xA8, 0xAD, 0xA8, 0xE1, 0xE2, 0xE0, 0xE0, 0xE8, 0xE0, 0xAD, 0xA8, 0xA5}
	got := decodeConsoleOutput(raw)
	if !strings.Contains(got, "Админ") {
		t.Fatalf("cp866 decode: %q", got)
	}
}
