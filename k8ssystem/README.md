
# Kubernetes

## Bugs|FRs|Limitations

+ Solution currently uses the default context

## Microk8s



## Kubernetes Engine

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
gcloud beta container cluster create ${NAME} \
--no-enable-basic-auth \
--release-channel="regular" \
--machine-type="f1-micro" \
--image-type="COS_CONTAINERD" \
--num-nodes="1" \
--enable-stackdriver-kubernetes \
--enable-ip-alias \
--addons HorizontalPodAutoscaling,HttpLoadBalancing \
--enable-autoupgrade \
--enable-autorepair
--region=${REGION} \
--project=${PROJECT}
```
This pulls the credentials into ${HOME}/.kube/config *and* set the default context
```
gcloud container clusters get-credentials ${NAME} --project=${PROJECT} --region=${REGION}
```

## Debugging

```bash
NODE=$(kubectl get nodes/hades-canyon --output=jsonpath="{.status.addresses[0].address}")
PORT=$(kubectl get services/bigmachine-00 --output=jsonpath="{.spec.ports[0].nodePort}")
curl https://:${NODE}:${PORT}
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