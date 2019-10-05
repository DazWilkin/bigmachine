package compute_engine

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/grailbio/bigmachine"
	compute "google.golang.org/api/compute/v1"
)

const (
	instanceType = "f1-micro"
	imageProject = "debian-cloud"
	imageFamily  = "debian-9"
)

var service *compute.Service

func NewClient(ctx context.Context) (err error) {
	service, err = compute.NewService(ctx)
	return
}
func Create(ctx context.Context, project, zone, name string) (*bigmachine.Machine, error) {
	log.Printf("[Instance:Create] %s: creating", name)
	instance := &compute.Instance{
		Name:        name,
		MachineType: fmt.Sprintf("projects/%s/zones/%s/machineTypes/%s", project, zone, instanceType),
		Disks: []*compute.AttachedDisk{
			&compute.AttachedDisk{
				AutoDelete: true,
				Boot:       true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					SourceImage: fmt.Sprintf("projects/%s/global/images/family/%s", imageProject, imageFamily),
				},
			},
		},
		Tags: &compute.Tags{
			Items: []string{
				"bigmachine",
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			&compute.NetworkInterface{
				AccessConfigs: []*compute.AccessConfig{
					&compute.AccessConfig{
						Type: "ONE_TO_ONE_NAT",
					},
				},
			},
		},
	}
	operation, err := service.Instances.Insert(project, zone, instance).Context(ctx).Do()

	start := time.Now()
	timeout := 5 * time.Second
	for operation.Status != "RUNNING" && time.Since(start) < timeout {
		log.Println(operation.Status)
		time.Sleep(250 * time.Millisecond)
		service.ZoneOperations.Get(project, zone, operation.Name).Context(ctx).Do()
	}
	if operation.Status != "RUNNING" {
		// timed-out
		log.Println("Instance didn't create")
	}

	// Now that the instance is stable
	instance, err = service.Instances.Get(project, zone, name).Context(ctx).Do()
	if err != nil {
		log.Printf("[gce:Start:go] %s: %s", name, err)
	}

	// TODO(dazwilkin) We lose ownership of the instance here !?
	return &bigmachine.Machine{
		Addr:     instance.NetworkInterfaces[0].AccessConfigs[0].NatIP,
		Maxprocs: 0,
		NoExec:   false,
	}, nil
}
