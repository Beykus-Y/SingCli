//go:build windows

package netstats

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	iphlpapi         = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetIfTable2  = iphlpapi.NewProc("GetIfTable2")
	procFreeMibTable = iphlpapi.NewProc("FreeMibTable")
)

func Read(mode Mode, tunName string) (Counters, bool, error) {
	adapters, err := readAdapters()
	if err != nil {
		return Counters{}, false, err
	}
	total, ok := Aggregate(adapters, mode, tunName)
	return total, ok, nil
}

func readAdapters() ([]AdapterSnapshot, error) {
	var table uintptr
	ret, _, err := procGetIfTable2.Call(uintptr(unsafe.Pointer(&table)))
	if ret != 0 {
		if err != windows.ERROR_SUCCESS {
			return nil, fmt.Errorf("GetIfTable2: %w", err)
		}
		return nil, fmt.Errorf("GetIfTable2 failed: %d", ret)
	}
	if table == 0 {
		return nil, nil
	}
	defer procFreeMibTable.Call(table)

	count := *(*uint32)(unsafe.Pointer(table))
	rowAlign := unsafe.Alignof(windows.MibIfRow2{})
	rowOffset := alignUp(unsafe.Sizeof(count), rowAlign)
	rowSize := unsafe.Sizeof(windows.MibIfRow2{})
	rows := table + rowOffset

	adapters := make([]AdapterSnapshot, 0, count)
	for i := uint32(0); i < count; i++ {
		row := (*windows.MibIfRow2)(unsafe.Pointer(rows + uintptr(i)*rowSize))
		adapters = append(adapters, AdapterSnapshot{
			Name:          windows.UTF16ToString(row.Alias[:]),
			Description:   windows.UTF16ToString(row.Description[:]),
			DownloadBytes: row.InOctets,
			UploadBytes:   row.OutOctets,
			Up:            row.OperStatus == windows.IfOperStatusUp,
			Loopback:      row.Type == uint32(windows.IF_TYPE_SOFTWARE_LOOPBACK),
		})
	}
	return adapters, nil
}

func alignUp(value uintptr, align uintptr) uintptr {
	if align == 0 {
		return value
	}
	remainder := value % align
	if remainder == 0 {
		return value
	}
	return value + align - remainder
}
