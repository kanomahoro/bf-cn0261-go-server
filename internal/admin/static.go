package admin

import _ "embed"

// indexHTML is the console page served at "/". It is embedded so the binary stays
// a single file, and it carries its own CSS and JavaScript (no build step).
//
//go:embed index.html
var indexHTML []byte
