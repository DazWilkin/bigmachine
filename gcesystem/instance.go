package gcesystem

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
	networkTag   = "bigmachine"
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

// TODO(dazwilkin) would it be preferable to represent this as a new type?
var service *compute.Service

// NewClient is a super-thin wrapper around Compute Engine's NewService call
func NewClient(ctx context.Context) (err error) {
	service, err = compute.NewService(ctx)
	return
}

// Create creates a Compute Engine instance returning a bigmachine.Machine
func Create(ctx context.Context, project, zone, name, image, authorityDir string) (*bigmachine.Machine, error) {
	if project == "" {
		return nil, fmt.Errorf("[Create] Requires a GCP Project ID")
	}
	if zone == "" {
		return nil, fmt.Errorf("[Create] Requires a zone identifier")
	}
	if name == "" {
		return nil, fmt.Errorf("[Create] Requires a (unique) machine name")
	}
	if image == "" {
		return nil, fmt.Errorf("[Create] Requires a Compute Engine image name")
	}
	if authorityDir == "" {
		return nil, fmt.Errorf("[Create] Requires an authority directory name")
	}
	log.Printf("[Create] %s: defining", name)
	log.Printf("[Create] %s: using bootstrap image: %s", name, image)
	manifest := &Manifest{Spec: Spec{
		Containers: []Container{
			Container{
				Name:  "gceboot",
				Image: image,
				// Required to run containers on privileged ports (<1024)
				SecurityContext: SecurityContext{
					Privileged: true,
				},
				VolumeMounts: []VolumeMount{
					VolumeMount{
						Name:      "tmpfs",
						MountPath: "/tmp",
						ReadOnly:  false,
					},
					VolumeMount{
						Name:      "authority",
						MountPath: "/" + authorityDir,
						ReadOnly:  true,
					},
				},
				Args: []string{"-log=debug"},
				Env: []Env{
					// GCP related
					// These could (should?) be obtained from the metadata service
					// GOOGLE_APPLICATION_CREDENTIALS is not set here because it will be obtained automatically from the metadata service
					Env{
						Name:  "PROJECT",
						Value: project,
					},
					Env{
						Name:  "ZONE",
						Value: zone,
					},
					// BigMachine related
					Env{
						Name:  "BIGMACHINE_MODE",
						Value: "machine",
					},
					Env{
						Name:  "BIGMACHINE_SYSTEM",
						Value: systemName,
					},
					Env{
						Name: "BIGMACHINE_ADDR",
						// TODO(dazwilkin) Dislike that this is a global variable in gcesystem namespace
						Value: fmt.Sprintf("0.0.0.0:%d", port),
					},
				},
			},
		},
		Volumes: []Volume{
			Volume{
				Name: "tmpfs",
				EmptyDir: EmptyDir{
					Medium: "Memory",
				},
			},
			Volume{
				Name: "authority",
				HostPath: HostPath{
					Path: "/tmp/" + authorityDir,
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
	projectNumber, err := ProjectNumber(ctx, project)
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
		// The Tag 'bigmachine' will be utilized by:
		// - the Delete call to identify which instances are to be deleted
		// - a potential firewall rule to permit traffic to this instance's port(s)
		Tags: &compute.Tags{
			Items: []string{
				networkTag,
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
				Email:  fmt.Sprintf("%d-compute@developer.gserviceaccount.com", projectNumber),
				Scopes: scopes,
			},
		},
	}
	log.Printf("[Create] %s: being created", name)
	operation, err := service.Instances.Insert(project, zone, instance).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	log.Printf("[Create] %s: tagged [HTTP|HTTPS] to be caught by default firewall rules", name)
	log.Printf("[Create] %s: Google Cloud Logging enabled", name)

	// Wait (or timeout) for the instance to be "RUNNING"
	start := time.Now()
	timeout := 5 * time.Second
	for operation.Status != "RUNNING" && time.Since(start) < timeout {
		log.Printf("[Create] %s: Sleeping -- status %s", name, operation.Status)
		time.Sleep(250 * time.Millisecond)
		service.ZoneOperations.Get(project, zone, operation.Name).Context(ctx).Do()
	}
	if operation.Status != "RUNNING" {
		// timed-out
		return nil, fmt.Errorf("[Create] %s: create unsuccessful -- timed-out", name)
	}

	// Wait (or timeout) for the instance to be assigned an external IP
	addr := ""
	start = time.Now()
	timeout = 5 * time.Second
	for addr == "" && time.Since(start) < timeout {
		instance, err = service.Instances.Get(project, zone, name).Context(ctx).Do()
		if err != nil {
			return nil, err
		}

		num := len(instance.NetworkInterfaces)
		if num == 0 {
			return nil, fmt.Errorf("[Create] %s: created but has no network interfaces", name)
		}
		if num > 1 {
			log.Printf("[Create] %s: multiple (%d) network interfaces are available, using first(0)", name, num)
		}

		addr = instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
		if addr == "" {
			log.Printf("[Create] %s: Sleeping -- external IP unavailable", name)
			time.Sleep(250 * time.Millisecond)
		}
	}
	// If we get here and the 1st NatIP remains unset, then the loop must have timed-out
	if addr == "" {
		return nil, fmt.Errorf("[Create] %s: created but unable to get obtain external IP", name)
	}

	log.Printf("[Create] %s: created (%s)", name, addr)
	return &bigmachine.Machine{
		Addr:     fmt.Sprintf("https://%s:%d", addr, port),
		Maxprocs: 1,
		NoExec:   false,
	}, nil
}

// Delete deletes a Compute Engine instance
func Delete(ctx context.Context, project, zone, name string) error {
	operation, err := service.Instances.Delete(project, zone, name).Context(ctx).Do()
	if err != nil {
		return err
	}

	// Wait (or timeout) for the instance to be "RUNNING"
	start := time.Now()
	timeout := 5 * time.Second
	for operation.Status != "RUNNING" && time.Since(start) < timeout {
		log.Printf("[Delete] %s: Sleeping -- status %s", name, operation.Status)
		time.Sleep(250 * time.Millisecond)
		service.ZoneOperations.Get(project, zone, operation.Name).Context(ctx).Do()
	}
	if operation.Status != "RUNNING" {
		// timed-out
		return fmt.Errorf("[Delete] %s: delete unsuccessful -- timed-out", name)
	}
	// Succeeded
	return nil
}

// TODO(dazwilkin) should this return []bigmachine.Machine to match Create?
func List(ctx context.Context, project, zone string) ([]string, error) {
	var result []string
	instancesList := service.Instances.List(project, zone)
	//.Filter("network.tags=" + networkTag) -- does not work with the API (https://issuetracker.google.com/issues/143463446)
	//.MaxResults(1) -- debugging-only forces pages to contain a single element to test paging

	// The following: invokes the lambda function for *each* page of results, errors on error
	// Within the lambda, iterating over all instances (in the page) permits
	// e.g. appending all the instance names to the results slice
	// See
	// https://godoc.org/google.golang.org/api/compute/v1#InstancesListCall.Pages
	// Golang example: https://cloud.google.com/compute/docs/reference/rest/v1/instances/list
	if err := instancesList.Pages(ctx, func(page *compute.InstanceList) error {
		for _, instance := range page.Items {
			log.Printf("[List] instance: %s", instance.Name)
			result = append(result, instance.Name)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}
