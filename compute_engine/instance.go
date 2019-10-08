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
	imageProject = "cos-cloud"
	imageFamily  = "cos-stable"
)

var (
	scopes = []string{
		"https://www.googleapis.com/auth/devstorage.read_only",
		"https://www.googleapis.com/auth/logging.write",
		"https://www.googleapis.com/auth/monitoring.write",
		"https://www.googleapis.com/auth/servicecontrol",
		"https://www.googleapis.com/auth/service.management.readonly",
		"https://www.googleapis.com/auth/trace.append",
	}
)

var service *compute.Service

func NewClient(ctx context.Context) (err error) {
	service, err = compute.NewService(ctx)
	return
}

// ProjectNumber returns a project number for a given project
// TODO(dazwilkin) Implement Project # lookup
func ProjectNumber(id string) (string, error) {
	if id == "bigmachine" {
		return "343398520240", nil
	}
	log.Println("Not correctly implemented!")
	return "", fmt.Errorf("ProjectNumber is not correctly implemented: returns default value for project ID 'bigmachine'")
}

// Create creates a Compute Engine instance returning a bigmachine.Machine
// TODO(dazwilkin) Nothing is installed on the Debian instance: should it be a Container OS? What bootstrap (container|binary)?
func Create(ctx context.Context, project, zone, name, image string) (*bigmachine.Machine, error) {
	log.Printf("[Instance:Create] %s: creating", name)
	log.Printf("[Instance:Create] %s: using bootstrap image: %s", name, image)
	manifest := &Manifest{Spec: Spec{
		Containers: []Container{
			Container{
				Name:  "gceboot",
				Image: image,
				Args:  []string{"-log=debug"},
				Env: []Env{
					Env{
						Name:  "BIGMACHINE_MODE",
						Value: "machine",
					},
					Env{
						Name:  "BIGMACHINE_SYSTEM",
						Value: "gce",
					},
					Env{
						Name:  "BIGMACHINE_ADDR",
						Value: ":8443",
					},
				},
			},
		},
	},
	}
	value, err := manifest.String()
	if err != nil {
		return nil, err
	}

	// Convert Google Project [ID --> number]
	projectNumber, err := ProjectNumber(project)
	if err != nil {
		return nil, err
	}

	instance := &compute.Instance{
		Name:        name,
		MachineType: fmt.Sprintf("projects/%s/zones/%s/machineTypes/%s", project, zone, instanceType),
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				&compute.MetadataItems{
					Key:   "gce-container-declaration",
					Value: &value,
				},
				// TOOD(dazwilkin) By this, Logging is always enabled
				&compute.MetadataItems{
					Key: "google-logging-enabled",
					// TODO(dazwilkin) Really!? How else to get a *string from a literal?
					Value: func(s string) *string { return &s }("true"),
				},
			},
		},
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
				"http-server",
				"https-server",
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
		ServiceAccounts: []*compute.ServiceAccount{
			&compute.ServiceAccount{
				// TODO(dazwilkin)
				Email:  fmt.Sprintf("%s-compute@developer.gserviceaccount.com", projectNumber),
				Scopes: scopes,
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
