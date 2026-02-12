package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: testhelper <mode> [args...]\n")
		os.Exit(1)
	}

	mode := os.Args[1]
	args := os.Args[2:]

	switch mode {
	case "echo":
		for _, a := range args {
			fmt.Println(a)
		}
	case "env":
		for _, name := range args {
			fmt.Println(os.Getenv(name))
		}
	case "output":
		for i := 0; i+1 < len(args); i += 2 {
			stream := args[i]
			msg := args[i+1]
			switch stream {
			case "stdout":
				fmt.Fprintln(os.Stdout, msg)
			case "stderr":
				fmt.Fprintln(os.Stderr, msg)
			}
		}
	case "exit":
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "exit requires exactly one argument\n")
			os.Exit(1)
		}
		code, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid exit code: %s\n", args[0])
			os.Exit(1)
		}
		os.Exit(code)
	case "sleep":
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "sleep requires exactly one argument\n")
			os.Exit(1)
		}
		ms, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid sleep duration: %s\n", args[0])
			os.Exit(1)
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
	case "cwd":
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "getwd: %s\n", err)
			os.Exit(1)
		}
		fmt.Println(dir)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s\n", mode)
		os.Exit(1)
	}
}
