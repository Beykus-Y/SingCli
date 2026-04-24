//go:build !windows

package sysproxy

func Enable(addr string) error {
	return nil
}

func Disable() error {
	return nil
}
