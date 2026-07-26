//go:build ogr && windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

const ogrExpiryMessage = "Демонстрационная версия Restaurant Finance OGR завершила работу. Срок тестирования истёк 21.07.2026 в 17:00 (МСК)."

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	messageBoxW       = user32.NewProc("MessageBoxW")
	ogrExpiryDeadline = time.Date(2026, time.July, 21, 17, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
)

func checkOGRExpiry() bool {
	now := time.Now().In(ogrExpiryDeadline.Location())
	if now.Before(ogrExpiryDeadline) {
		return true
	}

	showOGRExpiryMessage()
	return false
}

func showOGRExpiryMessage() {
	text, _ := syscall.UTF16PtrFromString(ogrExpiryMessage)
	title, _ := syscall.UTF16PtrFromString("Restaurant Finance OGR")
	messageBoxW.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
