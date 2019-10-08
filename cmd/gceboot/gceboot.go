package main

import (
	"flag"

	"github.com/grailbio/base/log"
	"github.com/grailbio/bigmachine"
	"github.com/grailbio/bigmachine/compute_engine"
)

func main() {
	log.AddFlags()
	flag.Parse()
	_ = bigmachine.Start(compute_engine.Instance)
	log.Fatal("[GCE] bigmachine.Start returned")
}
