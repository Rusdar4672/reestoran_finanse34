//go:build !ogr

package main

// checkOGRExpiry is disabled in the regular production build.
func checkOGRExpiry() bool {
	return true
}
