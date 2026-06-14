package cmd

import "testing"

func TestValidateOutput(t *testing.T) {
	for _, ok := range []string{"table", "json", "yaml"} {
		if err := validateOutput(ok); err != nil {
			t.Errorf("validateOutput(%q) unexpected error: %v", ok, err)
		}
	}
	if err := validateOutput("xml"); err == nil {
		t.Errorf("validateOutput(\"xml\") should error")
	}
}
