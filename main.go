package main

import (
	"flag"
	"os"

	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/app"
	"github.com/yuanjunliang/ai-mini-gateway/internal/runtime/buildinfo"
)

var (
	version         = buildinfo.DefaultVersion
	commit          = buildinfo.DefaultCommit
	contractVersion = buildinfo.DefaultContractVersion
)

func main() {
	app.Run(version, commit, contractVersion)
}

func init() {
	flag.CommandLine.SetOutput(os.Stdout)
}
