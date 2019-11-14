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
	"github.com/grailbio/bigmachine/k8ssystem"
)

var (
	systemFlag   = flag.String("bigm.system", "local", "system on which to run the bigmachine")
	instanceType = flag.String("bigm.ec2type", "m3.medium", "instance type with which to launch a bigmachine EC2 cluster")
	ondemand     = flag.Bool("bigm.ec2ondemand", false, "use ec2 on-demand instances instead of spot")
	kubeconfig   = flag.String("bigm.kubeconfig", ".kube/config", "location of Kubernetes configuration file")
	clusterName  = flag.String("bigm.clustername", "", "Kubernetes cluster name")
	namespace    = flag.String("bigm.namespace", "default", "Kubernetes namespace")
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
		sys = &gcesystem.System{
			// TOOD(dazwilkin) This should be configurable either through the command-line or environment
		}
	case "k8s":
		// TODO(dazwilkin) This should be configurable either through the command-line or environment
		sys = &k8ssystem.System{
			KubeConfig:  *kubeconfig,
			ClusterName: *clusterName,
			Namespace:   *namespace,
		}
	case "local":
	}
	b := bigmachine.Start(sys)
	b.HandleDebug(http.DefaultServeMux)
	return b
}
