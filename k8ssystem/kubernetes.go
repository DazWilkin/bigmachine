package k8ssystem

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/grailbio/bigmachine"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	name      = "bigmachine"
	namespace = "bigmachine"
)

func Create(ctx context.Context, clusterName, image string, count uint8) ([]*bigmachine.Machine, error) {
	h := homedir.HomeDir()
	kubeconfig := filepath.Join(h, ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	rqst := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(int32(count)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "bigmachine",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "bigmachine",
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

	// Create Deployment
	fmt.Println("Creating deployment...")
	_, err := deploymentsClient.Create(rqst)
	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: "app=bigmachine",
	})
	for _, pod := range pods {

	}
}
func int32Ptr(i int32) *int32 { return &i }
