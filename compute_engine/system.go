package compute_engine

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/grailbio/bigmachine"

	compute "google.golang.org/api/compute/v1"
)

const (
	systemName = "gce"
)
const (
	defaultNamePrefix   = "bigmachine"
	defaultInstanceType = "f1-micro"
	defaultImageProject = "debian-cloud"
	defaultImageFamily  = "debian-9"
)

var _ bigmachine.System = (*System)(nil)

var (
	system = new(System)
)

func init() {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		log.Fatal("Compute Engine backend uses Application Default Credentials. GOOGLE_APPLICATION_CREDENTIALS environment variable is unset")
	}
	bigmachine.RegisterSystem(systemName, new(System))
}

type System struct {
	ImageFamily  string
	ImageProject string
	InstanceType string
	OnDemand     bool
	ProjectID    string
	Zone         string
}

func (s *System) Exit(int) {

}
func (s *System) HTTPClient() *http.Client {
	// TODO(dazwilkin) not yet implement
	log.Println("[gce:HTTPClient] not yet implemented")
	return &http.Client{}

}
func (s *System) KeepaliveConfig() (period, timeout, rpcTimeout time.Duration) {
	period = time.Minute
	timeout = 10 * time.Minute
	rpcTimeout = 2 * time.Minute
	return
}
func (s *System) ListenAndServe(addr string, handle http.Handler) error {
	return nil
}
func (s *System) Main() error {
	return nil
}
func (s *System) Maxprocs() int {
	return 0
}

// Name returns the name of this system
func (s *System) Name() string {
	return systemName
}
func (s *System) Init(b *bigmachine.B) error {
	log.Println("[gce:Init] Configure Compute Engine defaults")
	s.ProjectID = os.Getenv("PROJECT")
	s.Zone = os.Getenv("ZONE")
	s.ImageProject = defaultImageProject
	s.ImageFamily = defaultImageFamily
	s.InstanceType = defaultInstanceType
	return nil
}
func (s *System) Read(ctx context.Context, m *bigmachine.Machine, filename string) (io.Reader, error) {
	return nil, nil
}
func (s *System) Shutdown() {}

func (s *System) MachineType() string {
	return fmt.Sprintf("projects/%s/zones/%s/machineTypes/%s", s.ProjectID, s.Zone, s.InstanceType)
}
func (s *System) SourceImage() string {
	return fmt.Sprintf("projects/%s/global/images/family/%s", s.ImageProject, s.ImageFamily)
}

// Start attempts to create 'count' GCE instances returns a list of machines and (!) any failures
func (s *System) Start(ctx context.Context, count int) ([]*bigmachine.Machine, error) {
	if count == 0 {
		log.Println("gce:Start] warning: request to create 0 (zero) instances")
		return []*bigmachine.Machine{}, nil
	}
	if count < 0 {
		return nil, fmt.Errorf("unable to create <0 instances")
	}

	computeService, err := compute.NewService(ctx)
	if err != nil {
		return nil, err
	}

	type OperationError struct {
		machine *bigmachine.Machine
		err     error
	}
	var wg sync.WaitGroup

	// Create buffered (non-blocking) channel since we know the number of machines
	ch := make(chan OperationError, count)

	// Iterate over Machine creation writing results to the channel
	// Results are Operations or Errors
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			log.Printf("[gce:Start:go] %s: creating", name)
			instance := &compute.Instance{
				Name:        name,
				MachineType: s.MachineType(),
				Disks: []*compute.AttachedDisk{
					&compute.AttachedDisk{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							SourceImage: s.SourceImage(),
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
			// TODO(dazwilkin) Synchronous? The operation should complete (or fail) by the return
			operation, err := computeService.Instances.Insert(s.ProjectID, s.Zone, instance).Context(ctx).Do()

			log.Printf("[gce:Start:go] %s: %d status: %s", name, operation.Id, operation.Status)
			log.Println(err)

			// TODO(dazwilkin) What's the process to wait on a stable instance?
			start := time.Now()
			timeout := 1 * time.Minute
			for operation.Status != "RUNNING" && time.Since(start) < timeout {
				time.Sleep(1 * time.Second)
			}
			time.Sleep(5 * time.Second)

			// After Insert'ing need to Get
			instance, err = computeService.Instances.Get(s.ProjectID, s.Zone, name).Context(ctx).Do()
			if err != nil {
				log.Println("[gce:Start:go] %s: %s", name, err)
			}

			natIP := instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
			log.Printf("[gce:Start:go] %s: NatIP: %s", name, natIP)

			ch <- OperationError{
				machine: &bigmachine.Machine{
					Addr:     natIP,
					Maxprocs: 0,
					NoExec:   false,
				},
				err: err,
			}
		}(fmt.Sprintf("%s-%02d", defaultNamePrefix, i))
	}
	wg.Wait()

	// Proccess the Operations|Errors creating Machines
	// If there are errors, there will be fewer than 'count' machines
	var machines []*bigmachine.Machine
	var failures uint
	for i := range ch {
		if i.err != nil {
			log.Println("[gce:Start:go] %s", err)
			failures = failures + 1
		}
		machines = append(machines, i.machine)
	}
	if failures > 0 {
		err = fmt.Errorf("[gcs:Start] %d/%d machines were not created", failures, count)
	}
	return machines, err
}
func (s *System) Tail(ctx context.Context, m *bigmachine.Machine) (io.Reader, error) {
	return nil, nil
}
