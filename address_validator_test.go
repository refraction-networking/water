package water

// package water instead of water_test to access unexported struct addressValidator and its unexported fields/methods

import "testing"

func Test_addressValidator_validate(t *testing.T) {
	var a addressValidator

	// test catchAll with nil denylist
	a.catchAll = true

	if err := a.validate("random net", "random address"); err != ErrAddressValidatorNotInitialized {
		t.Errorf("Expected ErrAddressValidatorNotInitialized, got %v", err)
	}

	// test nil denylist entry
	a.denylist = map[string][]string{
		"denied address": nil,
	}

	if err := a.validate("random net", "denied address"); err != ErrAddressValidatorNotInitialized {
		t.Errorf("Expected ErrAddressValidatorNotInitialized, got %v", err)
	}

	// test denied address on denied network
	a.denylist["denied address"] = []string{"denied net"}

	if err := a.validate("denied net", "denied address"); err != ErrAddressValidationDenied {
		t.Errorf("Expected ErrAddressValidationDenied, got %v", err)
	}

	// test random network with denied address
	if err := a.validate("random net", "denied address"); err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	// test random address on denied network
	if err := a.validate("denied net", "random address"); err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	// test not catchAll with nil allowlist
	a.catchAll = false

	if err := a.validate("random net", "random address"); err != ErrAddressValidatorNotInitialized {
		t.Errorf("Expected ErrAddressValidatorNotInitialized, got %v", err)
	}

	// test nil allowlist entry
	a.allowlist = map[string][]string{
		"allowed address": nil,
	}

	if err := a.validate("random net", "allowed address"); err != ErrAddressValidatorNotInitialized {
		t.Errorf("Expected ErrAddressValidatorNotInitialized, got %v", err)
	}

	// test allowed address on allowed network
	a.allowlist["allowed address"] = []string{"allowed net"}

	if err := a.validate("allowed net", "allowed address"); err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	// test random network with allowed address
	if err := a.validate("random net", "allowed address"); err != ErrAddressValidationDenied {
		t.Errorf("Expected ErrAddressValidationDenied, got %v", err)
	}

	// test random address on allowed network
	if err := a.validate("allowed net", "random address"); err != ErrAddressValidationDenied {
		t.Errorf("Expected ErrAddressValidationDenied, got %v", err)
	}
}
