package host

import (
	"encoding/json"
	"testing"
)

// A turn of the loop runs with gofi's server alone.
func TestMCPConfigIsGofisServerAlone(t *testing.T) {
	var doc struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(MCPConfig()), &doc); err != nil || len(doc.Servers) != 1 || doc.Servers[ServerName]["command"] != "gofi" {
		t.Errorf("%s: %v", MCPConfig(), err)
	}
}
