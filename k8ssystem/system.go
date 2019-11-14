package k8ssystem

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/grailbio/base/sync/once"
	"github.com/grailbio/bigmachine"
	"github.com/grailbio/bigmachine/internal/authority"

	"golang.org/x/net/http2"
)

const (
	port       = 443
	systemName = "k8s"
)
const (
	httpTimeout = 30 * time.Second
)
const (
	authorityDir = "secrets"
	authorityCrt = "bigmachine.pem"
)

var (
	// Ensure that System implements the bigmachine.System interface
	_ bigmachine.System = (*System)(nil)
)
var (
	Instance = new(System)
)

func init() {
	bigmachine.RegisterSystem(systemName, new(System))
}

type System struct {
	KubeConfig        string
	ClusterName       string
	Namespace         string
	BootstrapImage    string
	authority         *authority.T
	authorityContents []byte
	clientOnce        once.Task
	clientConfig      *tls.Config
}

func (System) Exit(code int) {
	log.Print("[k8s:Exit] Entered")
	os.Exit(code)
}
func (s *System) HTTPClient() *http.Client {
	log.Print("[k8s:HTTPClient] Entered")
	err := s.clientOnce.Do(func() (err error) {
		s.clientConfig, _, err = s.authority.HTTPSConfig()
		return
	})
	if err != nil {
		log.Fatal(err)
	}
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: httpTimeout,
		}).DialContext,
		TLSClientConfig:     s.clientConfig,
		TLSHandshakeTimeout: httpTimeout,
	}
	http2.ConfigureTransport(transport)
	return &http.Client{Transport: transport}
}
func (s *System) Init(b *bigmachine.B) error {
	log.Print("[k8s:Init] Entered")

	s.BootstrapImage = fmt.Sprintf("%s:%s", os.Getenv("IMG"), os.Getenv("TAG"))

	// Mimicking  ec2machine.go implementation
	var err error
	if _, err := os.Stat(authorityDir); os.IsNotExist(err) {
		if err := os.Mkdir(authorityDir, 0755); err != nil {
			return err
		}
	}
	authorityFilename := filepath.Join(authorityDir, authorityCrt)
	s.authority, err = authority.New(authorityFilename)
	if err != nil {
		return err
	}

	log.Printf("[k8s:Init] Reading %s", authorityFilename)
	s.authorityContents, err = ioutil.ReadFile(authorityFilename)
	if err != nil {
		log.Printf("[k8s:Init] Error Reading %s", authorityFilename)
	}
	return err
}
func (s *System) KeepaliveConfig() (period, timeout, rpcTimeout time.Duration) {
	log.Print("[k8s:KeepAliveConfig] Entered")
	period = time.Minute
	timeout = 10 * time.Minute
	rpcTimeout = 2 * time.Minute
	return
}
func (s *System) ListenAndServe(addr string, handler http.Handler) error {
	log.Print("[k8s:ListenAndServe] Entered")
	if addr == "" {
		log.Printf("[k8s:ListenAndServe] No address provided")
		// TODO(dazwilkin) should this be ":" or "0.0.0.0:"?
		addr = fmt.Sprintf(":%d", port)
	}
	log.Printf("[k8s:ListenAndServe] Address: %s", addr)
	// TODO(dazwilkin) is this the first time that we could determine the port?
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	i, err := strconv.Atoi(p)
	if err != nil {
		return err
	}
	if i < 1024 {
		log.Printf("[k8s:ListenAndServe] Serving on a privileged port (%d) -- if this fails, check firewalls, Dockerfile(USER) etc.", i)
	}
	_, config, err := s.authority.HTTPSConfig()
	if err != nil {
		return err
	}
	config.ClientAuth = tls.RequireAndVerifyClientCert
	server := &http.Server{
		TLSConfig: config,
		Addr:      addr,
		Handler:   handler,
	}
	http2.ConfigureServer(server, &http2.Server{
		// MaxConcurrentStreams: maxConcurrentStreams,
	})
	return server.ListenAndServeTLS("", "")
}
func (s *System) Main() error {
	log.Print("[k8s:Main] Entered")
	return http.ListenAndServe(":3333", nil)
}

// MaxProcs returns the number of vCPUs in the instance
// TODO(dazwilkin) Implement MaxProcs so that it returns the actual number of vCPUs on the instance
func (s *System) Maxprocs() int {
	log.Print("[k8s:Maxprocs] Entered")
	log.Print("[k8s:Maxprocs] Return constant value (1) -- implement to return actual vCPUs")
	return 1
}

// Name returns the name of this system
func (s *System) Name() string {
	log.Print("[k8s:Name] Entered")
	return systemName
}
func (s *System) Read(ctx context.Context, m *bigmachine.Machine, filename string) (io.Reader, error) {
	log.Print("[k8s:Read] Entered")
	return nil, nil
}
func (s *System) Shutdown() {
	log.Print("[k8s:Shutdown] Entered")
}

// Start attempts to create 'count' Pods (!) (on distinct Nodes?) returning a list of machines and any failures
// TODO(dazwilkin) This should be a single kubectl --replicas=count call equivalent
func (s *System) Start(ctx context.Context, count int) ([]*bigmachine.Machine, error) {
	log.Print("[k8s:Start] Entered")
	if count < 0 {
		return nil, fmt.Errorf("unable to create negative number of machines")
	}
	if count > 256 {
		return nil, fmt.Errorf("unable to create more than 256 machines")
	}

	err := NewClient(ctx, s.KubeConfig)
	if err != nil {
		return nil, err
	}

	// Create the namespace if it doesn't exist
	err = Namespace(ctx, s.ClusterName, s.Namespace)
	if err != nil {
		// Irrecoverable: if we're unable to create the Namespace, we're unable to proceed
		return nil, err
	}

	// One benefit w/ Kubernetes is that we can create a Secret with the Authority file now and only once
	err = Secret(ctx, s.ClusterName, s.Namespace, prefix, s.authorityContents)
	if err != nil {
		// Irrecoverable: if we're unable to create the Secret, we'll be unable to create the Deployment (depends on the volume-mounted Secret)
		return nil, err
	}

	type Result struct {
		machine *bigmachine.Machine
		err     error
	}
	var wg sync.WaitGroup

	// Create buffered (non-blocking) channel since we know the number of machines
	// Results will either be success(bigmachine.Machine) or error
	ch := make(chan Result, count)

	// Iterate over Machine creation writing results to the channel
	// Results are Operations or Errors
	for i := 0; i < count; i++ {
		wg.Add(1)
		// TODO(dazwilkin) Convenient (during testing) to name this way; can't create more if existing instances haven't been deleted
		name := fmt.Sprintf("%s-%02d", prefix, i)
		go func(name string) {
			defer wg.Done()
			machine, err := Create(ctx, s.ClusterName, s.Namespace, name, s.BootstrapImage)
			ch <- Result{
				machine: machine,
				err:     err,
			}
		}(name)
	}
	log.Print("[k8s:Start] await completion of Go routines")
	wg.Wait()
	log.Print("[k8s:Start] Go routines have completed")
	close(ch)

	// Proccess the channel of Results
	// If there were errors, there will be fewer than 'count' machines
	var machines []*bigmachine.Machine
	var failures uint
	log.Print("[k8s:Start] Iterate over the channel")
	for i := range ch {
		if i.err != nil {
			log.Printf("[k8s:Start:go] %+v", i.err)
			failures = failures + 1
		} else {
			log.Printf("[k8s:Start] Adding bigmachine (%s)", i.machine.Addr)
			machines = append(machines, i.machine)
		}
	}
	log.Print("[k8s:Start] Done w/ channel")
	if failures == uint(count) {
		// Failed to create any machines; unrecoverable
		return nil, fmt.Errorf("[k8s:Start] Failed to create any machines")
	}
	if failures > 0 {
		// Failed to create some machines; recoverable
		err = fmt.Errorf("[k8s:Start] %d/%d machines were not created", failures, count)
	}
	log.Print("[k8s:Start] Completed")
	return machines, nil
}

// Tail attempts to tail the (remote?) bigmachine's logs
// TODO(dazwilkin) this should be straightforward w/ kubectl get logs ...
func (s *System) Tail(ctx context.Context, m *bigmachine.Machine) (io.Reader, error) {
	log.Print("[k8s:Tail] Entered")
	// Convert bigmachine.Machine --> Service
	// The only identifier we have for the Kubernetes resources is the machine's address
	u, err := url.Parse(m.Addr)
	if err != nil {
		return nil, err
	}
	p := u.Port()
	name, err := Lookup(ctx, s.ClusterName, s.Namespace, p)
	if err != nil {
		return nil, err
	}
	return Logs(ctx, s.ClusterName, s.Namespace, name)
}
