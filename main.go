package main

import (
	"github.com/hongnoul/mangap/cmd"
	"github.com/hongnoul/mangap/config"
	"github.com/hongnoul/mangap/log"
	"github.com/samber/lo"
)

func main() {
	lo.Must0(config.Setup())
	lo.Must0(log.Setup())
	cmd.Execute()
}
