//go:build !windows

package tunnel

func cleanupTun() {}

func isTunAddressAlreadyExists(err error) bool {
	return false
}
