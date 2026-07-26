package web

import "embed"

// Static содержит интерфейс, который используется и браузерным, и desktop-клиентом.
//
//go:embed static/*
var Static embed.FS
