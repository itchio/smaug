# smaug

[![CI](https://github.com/itchio/smaug/actions/workflows/test.yml/badge.svg)](https://github.com/itchio/smaug/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/itchio/smaug)](https://goreportcard.com/report/github.com/itchio/smaug)
[![Go Reference](https://pkg.go.dev/badge/github.com/itchio/smaug.svg)](https://pkg.go.dev/github.com/itchio/smaug)
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
