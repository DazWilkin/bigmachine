package k8ssystem

import (
	"context"
	"fmt"
	"io"
	"log"
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
const (
	microk8s = true
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
func Create(ctx context.Context, clusterName, namespace, name, image string) (*bigmachine.Machine, error) {
	log.Print("[k8s:Create] Entered")
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
							Name: "secrets",
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
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "tmp",
									ReadOnly:  false,
									MountPath: "/tmp",
								},
								{
									Name:      "secrets",
									ReadOnly:  true,
									MountPath: "/secrets",
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
				if microk8s {
					log.Print("[k8s:Created] Running on microk8s: using --type=NodePort")
					return apiv1.ServiceTypeNodePort
				}
				log.Print("[k8s:Created] Not running on microk8s: using --type=LoadBalancer")
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

	// NodePorts are provisioned "immediately"
	port, err := func(s apiv1.ServiceSpec) (int32, error) {
		if len(s.Ports) == 0 {
			return 0, fmt.Errorf("Unable to determine NodePort; no ports found")
		}
		if len(s.Ports) > 1 {
			return 0, fmt.Errorf("Unable to determine which (of %d) NodePorts to use", len(s.Ports))
		}
		if s.Ports[0].NodePort == 0 {
			return 0, fmt.Errorf("NodePort is set to zero")
		}
		return s.Ports[0].NodePort, nil
	}(sResp.Spec)
	if err != nil {
		log.Printf("[k8s:Create] %s/%s: unable to determine NodePort", namespace, name)
		return nil, err
	}
	if port == 0 {
		return nil, fmt.Errorf("Nodeport is zero")
	}
	log.Printf("[k8s:Create] %s/%s: service NodePort==%d", namespace, name, port)

	var host string
	if microk8s {
		host = "localhost"
	} else {
		// LoadBalancer provisioning takes time and we can't create the bigmachine.Machine until this succeeds
		start := time.Now()
		timeout := 30 * time.Second
		// While there are:
		// + no errors
		// + not timed out
		// + service returns "pending" load-balancer configuration
		for retries := 0; err == nil && time.Since(start) < timeout && sResp.Status.LoadBalancer.Ingress == nil; retries++ {
			log.Printf("[k8s:Create] %s/%s: awaiting Load-Balancer", name, namespace)
			time.Sleep(5 * time.Second)
			sResp, err = c.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
		}
		// Timed out without identifying a Load-balancer
		if err == nil && sResp.Status.LoadBalancer.Ingress == nil {
			log.Printf("[k8s:Create] %s/%s: unable to provision Load-Balancer before timeout", name, namespace)
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
	}
	addr := fmt.Sprintf("https://%s:%d", host, port)
	log.Printf("[k8s:Create] %s/%s: Load-balancer provisioned on %s", namespace, name, addr)

	return &bigmachine.Machine{
		Addr:     addr,
		Maxprocs: 1,
		NoExec:   false,
	}, nil
}

// Logs returns a streaming reader to the logs from the Pod associated with the given (Service) name
func Logs(ctx context.Context, clusterName, namespace, name string) (io.Reader, error) {
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
func Lookup(ctx context.Context, clusterName, namespace, port string) (string, error) {
	log.Print("[k8s:Lookup] Entered")
	log.Printf("[k8s:Lookup] Finding service with NodePort==%s", port)

	// TODO(dazwilkin) --selector=spec.ports[0].nodePort=port does not appear (!) to work
	opts := metav1.ListOptions{
		LabelSelector: "app=bigmachine",
		// FieldSelector: fmt.Sprintf("spec.ports[0].nodePort=%s", port),
	}
	// Returns a list of services
	sResp, err := c.CoreV1().Services(namespace).List(opts)
	if err != nil {
		return "", err
	}
	if len(sResp.Items) == 0 {
		return "", fmt.Errorf("no service was found")
	}

	// TODO(dazwilkin) --selector=spec.ports[0].nodePort=port does not appear (!) to work
	// if len(sResp.Items) > 1 {
	// 	return "", fmt.Errorf("multiple services were found; expected only one")
	// }
	// return sResp.Items[0].GetName(), nil

	// TODO(dazwilkin) --selector=spec.ports[0].nodePort=port does not appear (!) to work
	var serviceName string
	p, _ := strconv.Atoi(port)
	q := int32(p)
	for i := 0; i < len(sResp.Items) || serviceName == ""; i++ {
		if sResp.Items[i].Spec.Ports[0].NodePort == q {
			serviceName = sResp.Items[i].GetObjectMeta().GetName()
		}
	}
	return serviceName, nil
}

// Namespace creates a Kubernetes Namespace object
// This function possibly needn't check whether the namespace exists beforehand
func Namespace(ctx context.Context, clusterName, name string) error {
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
func Secret(ctx context.Context, clusterName, namespace, name string, data []byte) error {
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
