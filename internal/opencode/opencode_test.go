package opencode

import (
	"testing"
)

func TestClientInterface(t *testing.T) {
	c := &Client{}
	if c == nil {
		t.Error("Client is nil")
	}
}
