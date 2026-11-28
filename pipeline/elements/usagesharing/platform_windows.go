//go:build windows

/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence
 * (EUPL) v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */

package usagesharing

import (
	"fmt"
	"syscall"
	"unsafe"
)

// osVersionInfoEx is RTL_OSVERSIONINFOEXW.
// See https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/wdm/ns-wdm-_osversioninfoexw
type osVersionInfoEx struct {
	size             uint32
	majorVersion     uint32
	minorVersion     uint32
	buildNumber      uint32
	platformID       uint32
	csdVersion       [128]uint16
	servicePackMajor uint16
	servicePackMinor uint16
	suiteMask        uint16
	productType      byte
	reserved         byte
}

// rtlGetVersion gives the version Windows is, where GetVersion gives the
// version the program says it was made for. ntdll.dll is one of the DLLs
// Windows always loads from its system folder.
var rtlGetVersion = syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion")

// osVersion gives the Windows version and build, such as 10.0.26100, with
// the service pack where there is one, or empty where it cannot be read.
func osVersion() string {
	if rtlGetVersion.Find() != nil {
		return ""
	}
	info := osVersionInfoEx{}
	info.size = uint32(unsafe.Sizeof(info))
	if status, _, _ := rtlGetVersion.Call(uintptr(unsafe.Pointer(&info))); status != 0 {
		return ""
	}
	version := fmt.Sprintf("%d.%d.%d", info.majorVersion, info.minorVersion, info.buildNumber)
	if servicePack := syscall.UTF16ToString(info.csdVersion[:]); servicePack != "" {
		version += " " + servicePack
	}
	return version
}
