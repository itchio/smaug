# smaug

[![build status](https://git.itch.ovh/itchio/smaug/badges/master/build.svg)](https://git.itch.ovh/itchio/smaug/commits/master)
[![codecov](https://codecov.io/gh/itchio/smaug/branch/master/graph/badge.svg)](https://codecov.io/gh/itchio/smaug)
[![Go Report Card](https://goreportcard.com/badge/github.com/itchio/smaug)](https://goreportcard.com/report/github.com/itchio/smaug)
[![GoDoc](https://godoc.org/github.com/itchio/smaug?status.svg)](https://godoc.org/github.com/itchio/smaug)
[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/itchio/smaug/blob/master/LICENSE)

smaug contains utilities for running processes:

  * ...tied to a context (like `exec.CommandWithContext`)
  * ...in a process group (so a whole process tree can be waited on or killed)
  * ...optionally in a sandbox, such as:
    * firejail on Linux
    * sandbox-exec on macOS
    * a separate user on Windows (see `fuji`)

## License

Licensed under MIT License, see `LICENSE` for details.
