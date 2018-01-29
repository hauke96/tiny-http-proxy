package main

import (
	"io"
	"io/ioutil"
	"log"
	"os"
)

var InfoWriter io.Writer = os.Stdout
var ErrorWriter io.Writer = os.Stderr
var FatalWriter io.Writer = os.Stderr
var DebugWriter io.Writer = os.Stdout
var TraceWriter io.Writer = ioutil.Discard

const Metadata int = log.Ldate | log.Ltime | log.Lshortfile

var Trace *log.Logger = log.New(TraceWriter, "TRACE: ", Metadata)
var Error *log.Logger = log.New(TraceWriter, "ERROR: ", Metadata)
var Fatal *log.Logger = log.New(TraceWriter, "FATAL: ", Metadata)
var Debug *log.Logger = log.New(TraceWriter, "DEBUG: ", Metadata)
var Info *log.Logger = log.New(TraceWriter, "INFO : ", Metadata)
