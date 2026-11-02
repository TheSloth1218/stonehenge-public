package dashboard

import "embed"

// StaticFiles contains the built-in dashboard assets.
//
//go:embed static/*
var StaticFiles embed.FS
