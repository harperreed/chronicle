# Gotchas

Traps and decisions for people and agents working on chronicle. Dated entries; edit an entry when it changes rather than adding a second one.

- 2026-11-12: Publish chronicle to Homebrew as a formula, never a cask. Homebrew puts `com.apple.quarantine` on cask downloads, and our macOS binaries are only ad-hoc signed (not notarized), so Gatekeeper holds the binary at launch: every command, even `--help` and agents' `chronicle mcp`, hangs with no output. v1.8.1–v1.9.0 shipped as a cask and hit this; v1.9.1 went back to `Formula/chronicle.rb`, and `internal/releasepolicy` pins formula-only publishing. A hung `chronicle` on a Mac: check `xattr -l` on the real binary for the quarantine flag, then switch from the cask (README, Homebrew section). Moving back to a cask needs notarized binaries first.
