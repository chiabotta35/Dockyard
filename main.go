package main

import (
	"github.com/sirupsen/logrus"

	"github.com/dockyard/dockyard/cmd"
)

func init() {
	logrus.SetLevel(logrus.InfoLevel)
}

func main() {
	cmd.Execute()
}
