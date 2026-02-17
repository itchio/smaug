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

blacklist ${HOME}/.config/chromium
blacklist ${HOME}/.config/chrome
blacklist ${HOME}/.config/google-chrome
blacklist ${HOME}/.config/BraveSoftware
blacklist ${HOME}/.config/vivaldi
blacklist ${HOME}/.config/microsoft-edge
blacklist ${HOME}/.mozilla
blacklist ${HOME}/.ssh
blacklist ${HOME}/.gnupg
blacklist ${HOME}/.aws
blacklist ${HOME}/.kube
blacklist ${HOME}/.pki
blacklist ${HOME}/.git-credentials
blacklist ${HOME}/.netrc
blacklist ${HOME}/.password-store
blacklist ${HOME}/.local/share/keyrings
`
