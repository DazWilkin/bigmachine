
# Kubernetes

## Installation

```Golang
case "k8s":
	sys = &k8ssystem.System{
		KubeConfig:   *kubeconfig,
		Namespace:    *namespace,
		LoadBalancer: *loadbalancer,
	}
```
E.g.:
```bash
go run github.com/grailbio/bigmachine/cmd/bigpi \
--bigm.system=k8s \
--bigm.namespace=${NAMESPACE} \
--bigm.loadbalancer=true \
--nmachine=2 \
--nsamples=100000
```

## Bugs|FRs|Limitations

+ A Kubernetes limitation makes it challenging to deploying the remote nodes as a single StatefulSet; it's not easy to individually access the Pods
+ Expects the kubeconfig `current-context` to be set and pointing to the cluster to be used
+ Creates TCP Load-balancer(s) only; consider supporting HTTPS Load-balancer when available
+ I suspect (!) service logs are not being tailed (correctly)

## [microk8s](https://microk8s.io)

```bash
SYSTEM=k8s
go run github.com/grailbio/bigmachine/cmd/bigpi \
--bigm.system=${SYSTEM} \
--bigm.namespace=${NAMESPACE} \
--bigm.loadbalancer=false \
--nmachine=16 \
--nsamples=100000
```
**NB** `--bigm.loadbalancer=false` because microk8s does not support service `--type=LoadBalancer`

## [Kubernetes Engine](https://cloud.google.com/kubernete-engine)

```bash
PROJECT=
```
Then:
```bash
gcloud services enable container.googleapis.com --project=${PROJECT}
```
And:
```bash
NAME=bigmachine
REGION=us-west1
gcloud beta container clusters create ${NAME} \
--no-enable-basic-auth \
--release-channel="regular" \
--machine-type="n1-standard-1" \
--image-type="COS_CONTAINERD" \
--num-nodes="1" \
--enable-stackdriver-kubernetes \
--enable-ip-alias \
--addons HorizontalPodAutoscaling,HttpLoadBalancing \
--enable-autoupgrade \
--enable-autorepair \
--region=${REGION} \
--project=${PROJECT}
```
This pulls the credentials into ${HOME}/.kube/config *and* sets the default context
```
gcloud container clusters get-credentials ${NAME} --project=${PROJECT} --region=${REGION}
```
then:
```bash
NAMESPACE=saturn
SYSTEM=k8s
go run github.com/grailbio/bigmachine/cmd/bigpi \
--bigm.system=${SYSTEM} \
--bigm.namespace=${NAMESPACE} \
--bigm.loadbalancer=true \
--nmachine=3 \
--nsamples=100000
```
**NB** Implementation creates one Load-balancer per `nmachine`; to save costs, keeping this number low(er)

You may monitor the Load-balancers being provisioned:
```bash
kubectl get services --namespace=saturn
NAME            TYPE           CLUSTER-IP   EXTERNAL-IP    PORT(S)         AGE
bigmachine-00   LoadBalancer   10.0.2.125   34.82.173.39   443:31425/TCP   91s
bigmachine-01   LoadBalancer   10.0.9.55    <pending>      443:32561/TCP   91s
bigmachine-02   LoadBalancer   10.0.10.12   <pending>      443:30226/TCP   91s
```
Then:
```bash
gcloud container clusters delete ${NAME} --project=${PROJECT} --region=${REGION} --quiet
```
**NB** This deletes the cluster's context from `~/.kube/config` as well
## [Digital Ocean Kubernetes](https://www.digitalocean.com/products/kubernetes/)

```bash
NAME=bigmachine
doctl kubernetes cluster create ${NAME} \
--auto-upgrade \
--node-pool="name=default;size=s-1vcpu-2gb;count=2" \
--region=sfo2 \
--update-kubeconfig \
--version=1.16.2-do.0 \
--wait=true
```
**NB** `--update-kubeconfig` fails (Snap?) on my machine; must edit these manually
then:
```bash
NAMESPACE=saturn
SYSTEM=k8s
go run github.com/grailbio/bigmachine/cmd/bigpi \
--bigm.system=${SYSTEM} \
--bigm.namespace=${NAMESPACE} \
--bigm.loadbalancer=true \
--nmachine=2 \
--nsamples=100000
```
**NB** Implementation creates one Load-balancer per `nmachine`; to save costs, keeping this number low(er)

```bash
kubectl get services --namespace=neptune
NAME            TYPE           CLUSTER-IP       EXTERNAL-IP       PORT(S)         AGE
bigmachine-00   LoadBalancer   10.245.131.167   157.230.199.197   443:31289/TCP   4m3s
bigmachine-01   LoadBalancer   10.245.75.196    134.209.142.48    443:32616/TCP   4m3s
```
and:
```bash
doctl compute load-balancer list --output=json | jq -r .[].ip
134.209.142.48
157.230.199.197
```
and:
If the solution fails to delete the Kubernetes namespace then the Load-balancers won't be deleted and must delete Load-balancer separately:
```bash
for LB in $(\
  doctl compute load-balancer list \
  --output=json \
  | jq -r .[].name)
do
  # async
  doctl compute load-balancer delete ${LB} --force &
done
```
```bash
doctl kubernetes cluster delete ${NAME} \
--update-kubeconfig \
--force
```

## Debugging

```bash
NODE=$(kubectl get nodes/hades-canyon --output=jsonpath="{.status.addresses[0].address}")
PORT=$(kubectl get services/bigmachine-00 --output=jsonpath="{.spec.ports[0].nodePort}")
curl https://:${NODE}:${PORT}
```

and:

```bash
POD=$(kubectl get pods --selector=app=bigmachine --namespace=${NAMESPACE} --output=jsonpath="{.metadata.name}" | shuf -n 1)

```

## Tidy

```bash
NAMESPACE=default
```
and
```bash
kubectl get secrets \
--selector=app=bigmachine \
--namespace=${NAMESPACE}
```
and
```bash
kubectl get deployments \
--selector=app=bigmachine \
--namespace=${NAMESPACE}
```
returns:
```
NAME            READY   UP-TO-DATE   AVAILABLE   AGE
bigmachine-00   0/1     1            0           35s
bigmachine-01   0/1     1            0           35s
bigmachine-02   0/1     1            0           35s
```
and:
```bash
kubectl get services \
--selector=app=bigmachine \
--namespace=${NAMESPACE}
```
returns:
```
NAME            TYPE       CLUSTER-IP       EXTERNAL-IP   PORT(S)         AGE
bigmachine-00   NodePort   10.152.183.2     <none>        443:31222/TCP   38s
bigmachine-01   NodePort   10.152.183.230   <none>        443:32016/TCP   40s
bigmachine-02   NodePort   10.152.183.44    <none>        443:32479/TCP   40s
```
and:
```bash
kubectl delete secret/bigmachine \
--selector=app=bigmachine \
--namespace=${NAMESPACE}

for S in $(\
  kubectl get services \
  --namespace=${NAMESPACE} \
  --selector=app=bigmachine \
  --output=jsonpath="{.items[*].metadata.name}")
do
  kubectl delete service/${S} \
  --namespace=${NAMESPACE}
done

for D in $(\
  kubectl get deployments \
  --namespace=${NAMESPACE} \
  --selector=app=bigmachine \
  --output=jsonpath="{.items[*].metadata.name}")
do
  kubectl delete deployment/${D} \
  --namespace=${NAMESPACE}
done
```