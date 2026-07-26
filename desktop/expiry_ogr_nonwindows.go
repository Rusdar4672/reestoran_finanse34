//go:build ogr && !windows

package main

func checkOGRExpiry() bool {
	return true
}
