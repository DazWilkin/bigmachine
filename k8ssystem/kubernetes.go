package k8ssystem

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"github.com/grailbio/bigmachine"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	prefix = "bigmachine"
)

var c *kubernetes.Clientset

func NewClient(ctx context.Context, kubeconfig string) (err error) {
	log.Print("[k8s:NewClient] Entered")
	if kubeconfig == "" {
		h := homedir.HomeDir()
		kubeconfig = filepath.Join(h, ".kube", "config")
	}
	log.Printf("[k8s:NewClient] %s", kubeconfig)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}
	c, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	return nil
}
func Create(ctx context.Context, namespace, name, image, authorityDir string, loadbalancer bool) (*bigmachine.Machine, error) {
	log.Printf("[k8s:Create] %s/%s: entered", namespace, name)
	// This should (!) be a Deployment|StatefulSet consistent 'count' replicas
	// But this make it challenging to expose each Pod as its own service
	// Each Pod needs to be its own service because this is how bigmachine operates
	// TODO(dazwilkin) Is there a way to achieve this using StatefulSets?
	// TODO(dazwilkin) Is there a better way to map bigmachine to Kubernetes?

	// Create Deployment for node 'j'
	dRqst := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app":  "bigmachine",
				"node": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "bigmachine",
					"node": name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":  "bigmachine",
						"node": name,
					},
				},
				Spec: apiv1.PodSpec{
					Volumes: []apiv1.Volume{
						{
							Name: "tmp",
							VolumeSource: apiv1.VolumeSource{
								EmptyDir: &apiv1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "authority",
							VolumeSource: apiv1.VolumeSource{
								Secret: &apiv1.SecretVolumeSource{
									SecretName: "bigmachine",
								},
							},
						},
					},
					Containers: []apiv1.Container{
						{
							Name:  "bigmachine",
							Image: image,
							Ports: []apiv1.ContainerPort{
								{
									Name:     "https",
									Protocol: apiv1.ProtocolTCP,
									// TODO(dazwilkin) global constant :-(
									ContainerPort: port,
								},
								// TODO(dazwilkin) Do I need this one?
								{
									Name:          "http",
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: 3333,
								},
							},
							Env: []apiv1.EnvVar{
								{
									Name:  "BIGMACHINE_MODE",
									Value: "machine",
								},
								{
									Name:  "BIGMACHINE_SYSTEM",
									Value: systemName,
								},
								{
									Name:  "BIGMACHINE_ADDR",
									Value: fmt.Sprintf("0.0.0.0:%d", port),
								},
							},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "tmp",
									ReadOnly:  false,
									MountPath: "/tmp",
								},
								{
									Name:      "authority",
									ReadOnly:  true,
									MountPath: "/" + authorityDir,
								},
							},
						},
					},
				},
			},
		},
	}
	log.Printf("[k8s:Create] %s/%s: creating Deployment", namespace, name)
	_, err := c.AppsV1().Deployments(namespace).Create(dRqst)
	if err != nil {
		return nil, err
	}

	// Create NodePort Service for the Deployment for node 'j'
	sRqst := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app":  "bigmachine",
				"node": name,
			},
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:     "https",
					Protocol: apiv1.ProtocolTCP,
					Port:     port,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: port,
					},
				},
			},
			Selector: map[string]string{
				"app":  "bigmachine",
				"node": name,
			},
			Type: func() apiv1.ServiceType {
				if !loadbalancer {
					log.Printf("[k8s:Create] %s/%s: running on a Kubernetes implementation without a TCP Load-balancer ('--type=NodePort')", namespace, name)
					return apiv1.ServiceTypeNodePort
				}
				log.Printf("[k8s:Create] %s/%s: running on a Kubernetes implementation with a TCP Load-balancer ('--type=LoadBalancer')", namespace, name)
				return apiv1.ServiceTypeLoadBalancer
			}(),
		},
	}
	log.Printf("[k8s:Create] %s/%s: creating Service", namespace, name)
	sResp, err := c.CoreV1().Services(namespace).Create(sRqst)
	if err != nil {
		return nil, err
	}
	log.Printf("[k8s:Create] %s/%s: service created", namespace, name)

	var host string
	var servicePort int32
	if loadbalancer {
		servicePort = port
		// LoadBalancer provisioning takes time and we can't create the bigmachine.Machine until this succeeds
		start := time.Now()
		timeout := 512 * time.Second
		backoff := 1 * time.Second
		log.Printf("[k8s:Create] %s/%s: provisioning TCP Load-balancer (timeout: %v)", namespace, name, timeout)
		// While there are:
		// + no errors
		// + not timed out
		// + service returns "pending" load-balancer configuration
		for retries := 0; err == nil && time.Since(start) < timeout && sResp.Status.LoadBalancer.Ingress == nil; retries++ {
			log.Printf("[k8s:Create] %s/%s: awaiting Load-Balancer (sleeping: %v)", namespace, name, backoff)
			time.Sleep(backoff)
			backoff *= 2
			sResp, err = c.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
		}
		// Timed out without identifying a Load-balancer
		if err == nil && sResp.Status.LoadBalancer.Ingress == nil {
			log.Printf("[k8s:Create] %s/%s: unable to provision Load-Balancer before timeout", namespace, name)
			return nil, fmt.Errorf("Unable to provision a load-balancer before timeout")
		}
		if err != nil && sResp.Status.LoadBalancer.Ingress != nil {
			// Something more untoward occurrred; it shouldn't because the service is provisioned.
			log.Printf("[k8s:Create] %s/%s: unexpected error occurred", name, namespace)
			return nil, err
		}
		// We either didn't time out or we got an error *but* we have what we want: a Load-balancer

		// We expect one-and-only-one Ingress object
		// Unsure if this path can occur, but...
		if len(sResp.Status.LoadBalancer.Ingress) == 0 {
			return nil, fmt.Errorf("no Load-balancer was created")
		}
		if len(sResp.Status.LoadBalancer.Ingress) > 1 {
			return nil, fmt.Errorf("multiple Load-balancers were created; only one was expected")
		}

		host = sResp.Status.LoadBalancer.Ingress[0].IP
		if host == "" {
			return nil, fmt.Errorf("Load-balancer was created but without an IP address")
		}
		// TODO(dazwilkin) improve this
		sleep := 90 * time.Second
		log.Printf("[k8s:Create] %s/%s: Giving the TCP Load-balancer time to stabilize (sleeping: %v)", namespace, name, sleep)
		time.Sleep(sleep)

	} else {
		// NodePorts are provisioned "immediately"
		servicePort = sResp.Spec.Ports[0].NodePort

		// TODO(dazwilkin) This should correctly return the external IP of (one of) the Node(s)
		host = "localhost"
		log.Printf("[k8s:Create] WARNING: Using '%s' as a Node IP ('--type=NodePort')", host)

	}
	addr := fmt.Sprintf("https://%s:%d", host, servicePort)
	log.Printf("[k8s:Create] %s/%s: endpoint %s", namespace, name, addr)

	log.Printf("[k8s:Create] %s/%s: completed", namespace, name)
	return &bigmachine.Machine{
		Addr:     addr,
		Maxprocs: 1,
		NoExec:   false,
	}, nil
}
func Delete(ctx context.Context, namespace string) error {
	log.Print("[k8s:Delete] Entered")
	// Deleting a namespace is the easiest way to delete everything in it
	// Unless the namespace is default :-(
	if namespace == "default" {
		log.Print("[k8s:Delete] Not yet implemented. Please delete everything for yourself")
		return fmt.Errorf("not yet implemented for the default namespace")
	}
	// Otherwise
	return c.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
}

// Logs returns a streaming reader to the logs from the Pod associated with the given (Service) name
func Logs(ctx context.Context, namespace, name string) (io.Reader, error) {
	log.Print("[k8s:Logs] Entered")

	// Need to identify this Service's Deployment's Pod(s)
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("node=%s", name),
	}
	// Returns a list of Pods
	pResp, err := c.CoreV1().Pods(namespace).List(opts)
	if err != nil {
		return nil, err
	}
	if len(pResp.Items) == 0 {
		return nil, fmt.Errorf("no pod was found")
	}
	if len(pResp.Items) > 1 {
		return nil, fmt.Errorf("multiple pods were found; expected only one")
	}
	podName := pResp.Items[0].GetObjectMeta().GetName()
	log.Printf("[k8s:Logs] streaming logs from %s/%s", namespace, podName)
	lRqst := c.CoreV1().Pods(namespace).GetLogs(podName, &apiv1.PodLogOptions{
		Container: "bigmachine",
		Follow:    true,
	})
	return lRqst.Stream()

}

// Lookup identifies the Kubernetes Service exposing the given (Node)Port
// This function is necessary because bigmachine.Machine are only identifiable by address (which contains a port)
// A complexity is that the Lookup is dependent on the service type (NodePort|LoadBalancer)
func Lookup(ctx context.Context, namespace, endpoint string, loadbalancer bool) (string, error) {
	log.Print("[k8s:Lookup] Entered")
	log.Printf("[k8s:Lookup] Finding service with endpoint==%s", endpoint)

	// We'll need to parse the endpoint
	u, err := url.Parse(endpoint)
	if err != nil {
		// if we can't, there's no need to proceed
		return "", err
	}

	// Our bigmachines are labelled
	opts := metav1.ListOptions{
		LabelSelector: "app=bigmachine",
	}
	// Returns a list of services corresponding to the bigmachines
	sResp, err := c.CoreV1().Services(namespace).List(opts)
	if err != nil {
		return "", err
	}
	if len(sResp.Items) == 0 {
		return "", fmt.Errorf("no service was found")
	}

	// Inline lambda that provides us with a function that matches (either --type=LoadBalancer|NodePort) against a service
	match := func(u *url.URL, loadbalancer bool) func(s apiv1.Service) bool {
		if loadbalancer {
			// If the --type=LoadBalancer then we need to return a function that matches the endpoint's host with the loadbalancer's address
			return func(s apiv1.Service) bool {
				return s.Status.LoadBalancer.Ingress[0].IP == u.Hostname()
			}
		} else {
			// If the --type=NodePort, then we need to return a function that matches the endpoint's port with the NodePort
			i, _ := strconv.Atoi(u.Port())
			nodeport := int32(i)
			return func(s apiv1.Service) bool {
				return s.Spec.Ports[0].NodePort == nodeport
			}
		}
	}(u, loadbalancer)

	var serviceName string
	// Iterate through the services until we're able to identify the service of interest
	for i := 0; i < len(sResp.Items) || serviceName == ""; i++ {
		if match(sResp.Items[i]) {
			serviceName = sResp.Items[i].GetObjectMeta().GetName()
		}
	}
	if serviceName == "" {
		return "", fmt.Errorf("unable to find the service")
	}
	return serviceName, nil
}

// Namespace creates a Kubernetes Namespace object
// This function possibly needn't check whether the namespace exists beforehand
func Namespace(ctx context.Context, name string) error {
	log.Printf("[k8s:Namespace] Checking existence of namespace (%s)", name)
	_, err := c.CoreV1().Namespaces().Get(name, metav1.GetOptions{})
	if err != nil {
		// Assume (?) namespace does not exist
		log.Printf("[k8s:Namespace] Namespace (%s) does not exist", name)
		nRqst := &apiv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: name,
				Labels: map[string]string{
					"app":  "bigmachine",
					"node": name,
				},
			},
		}
		log.Printf("[k8s:Namespace] Creating namespace (%s)", name)
		_, err := c.CoreV1().Namespaces().Create(nRqst)
		if err != nil {
			// Unrecoverable: unable to use the requested namespace
			return err
		}
	}
	log.Printf("[k8s:Namespace] Namespace (%s) available", name)
	return nil
}

// Secret creates a Kubernetes Secret object corresponding to the certificate generated by bigmachine
func Secret(ctx context.Context, namespace, name string, data []byte) error {
	log.Print("[k8s:Secret] Entered")
	s := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "bigmachine",
			},
		},
		Data: map[string][]byte{
			// TODO(dazwilkin) package constant :-(
			authorityCrt: data,
		},
		Type: apiv1.SecretTypeOpaque,
	}
	log.Printf("[k8s:Create] creating Secret (%s/%s)", namespace, name)
	_, err := c.CoreV1().Secrets(namespace).Create(s)
	return err
}
func int32Ptr(i int32) *int32 { return &i }
