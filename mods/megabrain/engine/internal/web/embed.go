package web

import "embed"

// Files contém a UI embutida no binário.
//
//go:embed all:static
var Files embed.FS
