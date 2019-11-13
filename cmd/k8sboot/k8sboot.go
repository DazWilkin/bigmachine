package main

import (
	"flag"

	"github.com/grailbio/base/log"
	"github.com/grailbio/bigmachine"
	"github.com/grailbio/bigmachine/k8ssystem"
)

func main() {
	log.AddFlags()
	flag.Parse()
	_ = bigmachine.Start(k8ssystem.Instance)
	log.Fatal("[Kubernetes] bigmachine.Start returned")
}
