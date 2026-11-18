//go:build warden

package mcp

import (
	"strings"
	"testing"
)

func TestWardenTagDoesNotLinkThePrivateModule(t *testing.T) {
	_, err := newWardenHook()
	if err == nil || !strings.Contains(err.Error(), "drover-warden") {
		t.Fatal(err)
	}
}
