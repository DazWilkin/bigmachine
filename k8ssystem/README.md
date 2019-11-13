
# Kubernetes

## Tidy

```bash
kubectl get deployments \
--selector=app=bigmachine \
--namespace=default

NAME            READY   UP-TO-DATE   AVAILABLE   AGE
bigmachine-00   0/1     1            0           35s
bigmachine-01   0/1     1            0           35s
bigmachine-02   0/1     1            0           35s

kubectl get services \
--selector=app=bigmachine \
--namespace=default

NAME            TYPE       CLUSTER-IP       EXTERNAL-IP   PORT(S)         AGE
bigmachine-00   NodePort   10.152.183.2     <none>        443:31222/TCP   38s
bigmachine-01   NodePort   10.152.183.230   <none>        443:32016/TCP   40s
bigmachine-02   NodePort   10.152.183.44    <none>        443:32479/TCP   40s
```
and:
```
for S in $(kubectl get services --namespace=default --selector=app=bigmachine --output=jsonpath="{.items[*].metadata.name}")
do
  kubectl delete service/${S} --namespace=default
done

for D in $(kubectl get deployments --namespace=default --selector=app=bigmachine --output=jsonpath="{.items[*].metadata.name}")
do
  kubectl delete deployment/${D} --namespace=default
done
```