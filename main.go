package main

import (
	_ "net/http/pprof"

	"github.com/easterthebunny/solana-slot-loader/cmd"
)

func main() {
	cmd.Execute()
}
