//go:build wasip1 || wasi

package addon

import (
	"fmt"
)

var username string = "anonymous"

func Login(name string) {
	username = name
}

// Exports a few functions to host

// Export to host.
//
//export whoami
func Whoami() {
	fmt.Println("Logged in as " + username)
}

// Export to host.
//
//export attack
func Attack() {
	fmt.Println("Attacking...")
	sendNuke(1)
}

// Export to host.
//
//export attack_max
func AttackMax() {
	fmt.Println("Attacking Max...")
	sendNuke(100)
}

// Export to host.
//
//export stop
func Stop() {
	fmt.Println("Stopping...")
	if cancelNuke() != 0 {
		panic("Failed to cancel nuke")
	}
}
