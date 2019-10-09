package main

import (
	"flag"

	"github.com/grailbio/base/log"
	"github.com/grailbio/bigmachine"
	"github.com/grailbio/bigmachine/gcesystem"
)

func main() {
	log.AddFlags()
	flag.Parse()
	_ = bigmachine.Start(gcesystem.Instance)
	log.Fatal("[GCE] bigmachine.Start returned")
}
