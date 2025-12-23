module github.com/harper/chronicle

go 1.24.0

toolchain go1.24.11

// Use self-hosted Charm fork for KV sync with read-only fallback support
replace github.com/charmbracelet/charm => github.com/2389-research/charm v0.17.0

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de
	github.com/charmbracelet/charm v0.0.0-00010101000000-000000000000
	github.com/fatih/color v1.18.0
	github.com/google/uuid v1.6.0
	github.com/modelcontextprotocol/go-sdk v1.1.0
	github.com/spf13/cobra v1.10.1
)

require (
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/caarlos0/env/v6 v6.10.1 // indirect
	github.com/calmh/randomart v1.1.0 // indirect
	github.com/charmbracelet/bubbles v0.20.0 // indirect
	github.com/charmbracelet/bubbletea v1.3.3 // indirect
	github.com/charmbracelet/keygen v0.5.1 // indirect
	github.com/charmbracelet/lipgloss v1.0.0 // indirect
	github.com/charmbracelet/log v0.2.2 // indirect
	github.com/charmbracelet/x/ansi v0.8.0 // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.1 // indirect
	github.com/google/jsonschema-go v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jacobsa/crypto v0.0.0-20190317225127-9f44e2d11115 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/go-app-paths v0.2.2 // indirect
	github.com/muesli/sasquatch v0.0.0-20200811221207-66979d92330a // indirect
	github.com/muesli/termenv v0.15.2 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/exp v0.0.0-20251125195548-87e1e737ad39 // indirect
	golang.org/x/oauth2 v0.33.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/term v0.38.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	modernc.org/libc v1.66.10 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.41.0 // indirect
)
