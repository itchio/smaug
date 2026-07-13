package policies

// This templates generates a sandbox policy file suitable for
// running relatively-untrusted apps via itch.
//
// Reference:
// https://reverse.put.as/wp-content/uploads/2011/09/Apple-Sandbox-Guide-v1.0.pdf
const SandboxExecTemplate = `
(version 1)
(deny default)

(allow file*
  (subpath "{{.UserLibrary}}/Application Support")
  (subpath "{{.UserLibrary}}/Preferences")
  (subpath "{{.UserLibrary}}/Logs")
  (subpath "{{.UserLibrary}}/Caches")
  (subpath "{{.UserLibrary}}/KeyBindings")
  (subpath "{{.UserLibrary}}/Saved Application State")

  ;; Ren'Py games save to ~/Library/RenPy/<game_name> by default
  ;; https://github.com/itchio/smaug/issues/7
  (subpath "{{.UserLibrary}}/RenPy")

{{if .LegacyCompatibility}}
  ;; Legacy mode keeps broad device access for compatibility.
  (subpath "/dev")
{{else}}
  ;; Balanced mode limits device access to common low-risk nodes.
  (literal "/dev/null")
  (literal "/dev/random")
  (literal "/dev/urandom")
{{end}}

  (subpath "/private/var/folders")
  (subpath "/var/folders" )
)

(deny file*
  (subpath "{{.UserLibrary}}/Application Support/itch")
  (subpath "{{.UserLibrary}}/Application Support/kitch")
  (subpath "{{.UserLibrary}}/Application Support/Google")
  (subpath "{{.UserLibrary}}/Application Support/Mozilla")
)

(allow file*
  ;; where the app is actually installed
  ;; note: the app won't be able to scan/access apps from other locations
  (subpath "{{.InstallLocation}}")
)

(allow file-read*
  ;; binaries & executables
  (subpath "/usr/local")
  (subpath "/usr/share")
  (subpath "/usr/lib")
  (subpath "/usr/bin")
  (subpath "/bin")
  (subpath "/System/Library")

  ;; Rosetta 2 translation (required for x86_64 binaries on Apple Silicon)
  (subpath "/usr/libexec/rosetta")
  (subpath "/Library/Apple/usr/libexec/oah")
  (subpath "/Library/Java/JavaVirtualMachines")

{{if .LegacyCompatibility}}
  ;; Legacy mode keeps a very broad /private read for compatibility.
  (subpath "/private")
{{end}}

  ;; preferences
  (subpath "/etc")
  (subpath "/private/etc")
  (subpath "/Library/Preferences")

  ;; resources
  (subpath "/Library/Audio")
  (subpath "/Library/Fonts")

  (subpath "{{.UserLibrary}}/Keyboard Layouts")
  (subpath "{{.UserLibrary}}/Input Methods")
  (subpath "{{.UserLibrary}}/Fonts")

  ;; FIXME that's a bit excessive, why are some apps
  ;; trying to read 'PkgInfo' files or 'rsrc' ?
  (subpath "/Applications")

  ;; Chrome Helper
  (literal "/Library/Application Support/CrashReporter/SubmitDiagInfo.domains")
  (literal "/")
)

;; You'd be surprised what some apps scan for some reason
(allow file-read-metadata)

;; threads + launching other binaries
(allow process-fork)
(allow process-exec)

;; probe hardware/OS limits? e.g. hw.pagesize_compat
(allow sysctl-read)

{{if .AllowNetwork}}
;; network
(allow network-bind)
(allow network-outbound)
{{end}}

;; (required by Electron/Chromium to load images, for example)
(allow system-socket)

;; (required by SDL2 app, was asking for 'com.apple.cfprefsd.daemon')
(allow mach-lookup)
(allow mach-register) ;; 'axserver, portname, CFPasteboardClient'

;; Shared memory read-writes
(allow ipc-posix*)

;; ?? (required by SDL2 app)
(allow iokit-open)
`
