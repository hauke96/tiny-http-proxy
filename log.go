package main

import (
	"io"
	"log"
	"os"
)

const Metadata int = log.Ldate | log.Lmicroseconds | log.Lshortfile

var InfoWriter io.Writer = os.Stdout
var DebugWriter io.Writer = os.Stdout
var ErrorWriter io.Writer = os.Stderr

var Info *log.Logger = log.New(InfoWriter, "INFO : ", Metadata)
var Debug *log.Logger = log.New(DebugWriter, "DEBUG: ", Metadata)
var Error *log.Logger = log.New(ErrorWriter, "ERROR: ", Metadata)
