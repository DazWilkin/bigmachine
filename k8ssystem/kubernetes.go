package k8ssystem

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

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
	// TODO(dazwilkin) should probably deploy to a user-defined namespace
	namespace = "default"
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
func Create(ctx context.Context, clusterName, name, image, authority string) (*bigmachine.Machine, error) {
	log.Print("[k8s:Create] Entered")
	// This should (!) be a Deployment|StatefulSet consistent 'count' replicas
	// But this make it challenging to expose each Pod as its own service
	// Each Pod needs to be its own service because this is how bigmachine operates
	// TODO(dazwilkin) Is there a way to achieve this using StatefulSets?
	// TODO(dazwilkin) Is there a better way to map bigmachine to Kubernetes?

	// Create Deployment for node 'j'
	d := &appsv1.Deployment{
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
					Containers: []apiv1.Container{
						{
							Name:  name,
							Image: image,
							Ports: []apiv1.ContainerPort{
								{
									Name:     "http",
									Protocol: apiv1.ProtocolTCP,
									// TODO(dazwilkin) global constant :-(
									ContainerPort: port,
								},
							},
						},
					},
				},
			},
		},
	}
	log.Printf("[k8s:Create] creating Deployment (%s)", name)
	_, err := c.AppsV1().Deployments(namespace).Create(d)
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
					Name:     "bigmachine",
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
			Type: apiv1.ServiceTypeNodePort,
		},
	}
	log.Printf("[k8s:Create] creating Service (%s)", name)
	sResp, err := c.CoreV1().Services(namespace).Create(sRqst)
	if err != nil {
		return nil, err
	}
	log.Printf("%v", sResp)

	addr, err := func(s apiv1.ServiceSpec) (string, error) {
		if s.ClusterIP == "" {
			return "", fmt.Errorf("Unable to determine ClusterIP")
		}
		if len(s.Ports) == 0 {
			return "", fmt.Errorf("Unable to determine NodePort; no ports found")
		}
		if len(s.Ports) > 1 {
			return "", fmt.Errorf("Unable to determine which (of %d) NodePorts to use", len(s.Ports))
		}
		if s.Ports[0].NodePort == 0 {
			return "", fmt.Errorf("NodePort is set to zero")
		}
		return fmt.Sprintf("%s:%d", s.ClusterIP, s.Ports[0].NodePort), nil
	}(sResp.Spec)
	if err != nil {
		return nil, err
	}

	return &bigmachine.Machine{
		Addr:     addr,
		Maxprocs: 1,
		NoExec:   false,
	}, nil
}
func int32Ptr(i int32) *int32 { return &i }
