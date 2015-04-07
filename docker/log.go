package main

import (
	"github.com/Sirupsen/logrus"
	"io"
)

//封装了来自logrus中的函数,提供log的功能

func setLogLevel(lvl logrus.Level) {
	logrus.SetLevel(lvl)
}

func initLogging(stderr io.Writer) {
	logrus.SetOutput(stderr)
}
