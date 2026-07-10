package installer

import "testing"

func TestParseDbUpdaterLine(t *testing.T) {
	msg, ok := parseDbUpdaterLine("[state]:15/120")
	if !ok || msg.Kind != dbUpdaterLineState || msg.Payload != "15/120" {
		t.Fatalf("state parse: ok=%v msg=%+v", ok, msg)
	}

	msg, ok = parseDbUpdaterLine("[result]:0")
	if !ok || msg.Kind != dbUpdaterLineResult || msg.Payload != "0" {
		t.Fatalf("result parse: ok=%v msg=%+v", ok, msg)
	}

	msg, ok = parseDbUpdaterLine("[err]:something failed")
	if !ok || msg.Kind != dbUpdaterLineError || msg.Payload != "something failed" {
		t.Fatalf("error parse: ok=%v msg=%+v", ok, msg)
	}
}

func TestDbUpdaterProgressPercent(t *testing.T) {
	pct, ok := dbUpdaterProgressPercent("15/120")
	if !ok || pct != 12 {
		t.Fatalf("expected 12%% got %d ok=%v", pct, ok)
	}
}

func TestDbUpdaterFinishedSuccessfully(t *testing.T) {
	if !dbUpdaterFinishedSuccessfully(nil, "0", true) {
		t.Fatal("expected success for result 0")
	}
	if dbUpdaterFinishedSuccessfully([]string{"x"}, "0", true) {
		t.Fatal("expected failure when errors present")
	}
	if dbUpdaterFinishedSuccessfully(nil, "-1", true) {
		t.Fatal("expected failure for result -1")
	}
}
