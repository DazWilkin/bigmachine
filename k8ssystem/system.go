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
	"os"
	"path/filepath"
	"strconv"
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
	ClusterName       string
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
	s.authorityContents, err = ioutil.ReadFile(authorityFilename)
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
		log.Printf("[gce:ListenAndServe] Serving on a privileged port (%d) -- if this fails, check firewalls, Dockerfile(USER) etc.", i)
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
	if count<0 
	machines, err := Create(ctx, s.ClusterName, s.BootstrapImage, uint8(count))
	return nil, nil
}

// Tail attempts to tail the (remote?) bigmachine's logs
// TODO(dazwilkin) this should be straightforward w/ kubectl get logs ...
func (s *System) Tail(ctx context.Context, m *bigmachine.Machine) (io.Reader, error) {
	log.Print("[k8s:Tail] Entered")
	return nil, nil
}
