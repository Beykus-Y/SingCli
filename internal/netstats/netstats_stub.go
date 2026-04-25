//go:build !windows

package netstats

func Read(mode Mode, tunName string) (Counters, bool, error) {
	return Counters{}, false, nil
}
