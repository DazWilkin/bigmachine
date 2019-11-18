// Copyright 2018 GRAIL, Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package driver provides a convenient API for bigmachine drivers,
// which includes configuration by flags. Driver exports the
// bigmachine's diagnostic http handlers on the default ServeMux.
//
//	func main() {
//		flag.Parse()
//		b := driver.Start()
//		defer b.shutdown()
//		// Driver code
//	}
package driver

import (
	"flag"
	"net/http"

	"github.com/grailbio/base/log"
	"github.com/grailbio/bigmachine"
	"github.com/grailbio/bigmachine/ec2system"
	"github.com/grailbio/bigmachine/gcesystem"
)

var (
	systemFlag   = flag.String("bigm.system", "local", "system on which to run the bigmachine")
	instanceType = flag.String("bigm.ec2type", "m3.medium", "instance type with which to launch a bigmachine EC2 cluster")
	ondemand     = flag.Bool("bigm.ec2ondemand", false, "use ec2 on-demand instances instead of spot")
	project      = flag.String("bigm.project", "", "Google Cloud Platform project ID")
	zone         = flag.String("bigm.zone", "", "Google Cloud Platform zone")
	image        = flag.String("bigm.image", "", "Bootstrap image to be deployed on remote nodes")
)

// Start configures a bigmachine System based on the program's flags,
// Sand then starts it. ee bigmachine.Start for more details.
func Start() *bigmachine.B {
	sys := bigmachine.Local
	switch *systemFlag {
	default:
		log.Fatalf("unrecognized system %s", *systemFlag)
	case "ec2":
		sys = &ec2system.System{
			InstanceType: *instanceType,
			OnDemand:     *ondemand,
		}
	case "gce":
		if *project == "" {
			log.Fatal("GCP Project ID ('--bigm.project') is required")
		}
		if *zone == "" {
			log.Fatal("GCP Zone ('--bigm.zone') is required")
		}
		if *image == "" {
			log.Fatal("Image ('--bigm.image') is required")
		}

		sys = &gcesystem.System{
			Project:        *project,
			Zone:           *zone,
			BootstrapImage: *image,
		}

	case "local":
	}
	b := bigmachine.Start(sys)
	b.HandleDebug(http.DefaultServeMux)
	return b
}
