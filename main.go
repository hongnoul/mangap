package main

import (
	"github.com/hongnoul/mangaz/cmd"
	"github.com/hongnoul/mangaz/config"
	"github.com/hongnoul/mangaz/log"
	"github.com/samber/lo"
)

func main() {
	lo.Must0(config.Setup())
	lo.Must0(log.Setup())
	cmd.Execute()
}
