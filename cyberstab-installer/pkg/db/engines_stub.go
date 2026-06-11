//go:build !linux

package db

func discoverAdditionalEngines(addBin func(string)) {}
