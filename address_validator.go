package water

import (
	"errors"
)

var (
	ErrAddressValidatorNotInitialized = errors.New("address validator not initialized properly")
	ErrAddressValidationDenied        = errors.New("address validation denied")
)

type addressValidator struct {
	catchAll  bool
	allowlist map[string][]string // map[address]networks
	denylist  map[string][]string // map[address]networks
}

func (a *addressValidator) validate(network, address string) error {
	if a.catchAll {
		// only check denylist, otherwise allow
		if a.denylist == nil {
			return ErrAddressValidatorNotInitialized
		}

		if deniedNetworks, ok := a.denylist[address]; ok {
			if deniedNetworks == nil {
				return ErrAddressValidatorNotInitialized
			}

			for _, deniedNet := range deniedNetworks {
				if deniedNet == network {
					return ErrAddressValidationDenied
				}
			}
		}
		return nil
	} else {
		// only check allowlist, otherwise deny
		if a.allowlist == nil {
			return ErrAddressValidatorNotInitialized
		}

		if allowedNetworks, ok := a.allowlist[address]; ok {
			if allowedNetworks == nil {
				return ErrAddressValidatorNotInitialized
			}

			for _, allowedNet := range allowedNetworks {
				if allowedNet == network {
					return nil
				}
			}
		}
		return ErrAddressValidationDenied
	}
}
