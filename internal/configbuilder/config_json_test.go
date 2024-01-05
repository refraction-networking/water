package configbuilder_test

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gaukas/water/internal/configbuilder"
)

var (
	//go:embed testdata/config_base64.json
	configB64 []byte
)

func TestConfigJSON(t *testing.T) {
	var configJSON configbuilder.ConfigJSON

	if err := json.Unmarshal(configB64, &configJSON); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	fmt.Println(configJSON)
}
