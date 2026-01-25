package functions

import (
	"testing"

	"github.com/extism/go-sdk"
)

// TestExtismSDKLoads verifies that the Extism Go SDK can be imported and basic types are available.
func TestExtismSDKLoads(t *testing.T) {
	// Just verify the SDK can be imported and key types are accessible
	_ = extism.NewPlugin
	_ = extism.Manifest{}
	_ = extism.PluginConfig{}
}
