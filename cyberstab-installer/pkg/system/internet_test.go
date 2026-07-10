package system

import "testing"

func TestHasInternetAccessDoesNotPanic(t *testing.T) {
	_ = HasInternetAccess()
}
