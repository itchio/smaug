package policies

// This templates generates a sandbox policy file suitable for
// running relatively-untrusted apps via itch.

const FirejailTemplate = `
include /etc/firejail/itch_game_{{.Name}}.local
include /etc/firejail/itch_games_globals.local

noblacklist {{.FullTargetPath}}
noblacklist {{.InstallFolder}}
noblacklist {{.TempDir}}
blacklist {{.InstallFolder}}/.itch

noblacklist ${HOME}/.config/itch/apps
blacklist   ${HOME}/.config/itch/*
blacklist   ${HOME}/.config/itch/apps/*

noblacklist ${HOME}/.config/kitch/apps
blacklist   ${HOME}/.config/kitch/*
blacklist   ${HOME}/.config/kitch/apps/*

blacklist ~/.config/chromium
blacklist ~/.config/chrome
blacklist ~/.mozilla
`
