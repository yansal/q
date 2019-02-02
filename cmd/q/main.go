package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
)

type subcmd struct {
	run   func() error
	usage string
}

var cmds map[string]subcmd

func init() {
	cmds = map[string]subcmd{
		"help":    {run: help, usage: "print help message"},
		"receive": {run: receive, usage: "run queue receiver"},
		"send":    {run: send, usage: "send a message to a queue"},
		"stats":   {run: stats, usage: "print stats"},
		"web":     {run: web, usage: "run dashboard web server"},
	}
}

func usage() {
	var names []string
	for name := range cmds {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Fprint(flag.CommandLine.Output(), "Usage:\n\n\tq <command> [arguments]\n\n")
	fmt.Fprint(flag.CommandLine.Output(), "Commands:\n\n")
	for _, name := range names {
		fmt.Fprintf(flag.CommandLine.Output(), "\t%-8s\t%s\n", name, cmds[name].usage)
	}
}

func help() error {
	usage()
	return nil
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("q: ")

	flag.Usage = usage
	flag.Parse()

	cmd, ok := cmds[flag.Arg(0)]
	if !ok {
		help()
		code := 2
		if len(flag.Arg(0)) == 0 {
			code = 0
		}
		os.Exit(code)
	}

	if err := cmd.run(); err != nil {
		log.Fatalf("%+v", err)
	}
}
