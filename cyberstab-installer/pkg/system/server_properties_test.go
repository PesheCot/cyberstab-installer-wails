package system

import "testing"

func TestParsePropertiesInt(t *testing.T) {
	data := []byte(`
# network
network.portnumber = 2017
management.portnumber = 2020
`)
	v, ok := parsePropertiesInt(data, "network.portnumber")
	if !ok || v != 2017 {
		t.Fatalf("network port: got %d ok=%v", v, ok)
	}
	v, ok = parsePropertiesInt(data, "management.portnumber")
	if !ok || v != 2020 {
		t.Fatalf("management port: got %d ok=%v", v, ok)
	}
}

func TestLoadServerPortsDefaultsWhenMissing(t *testing.T) {
	ports := LoadServerPorts("/nonexistent/path/for-test")
	if ports.NetworkPort != DefaultNetworkPort || ports.ManagementPort != DefaultManagementPort {
		t.Fatalf("defaults: %+v", ports)
	}
}
