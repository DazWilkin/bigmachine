# Kubernetes

```bash
REPO=gcr.io/bigmachine/k8sboot
TAG=$(git rev-parse HEAD)
docker build --tag=${REPO}:${TAG} --file=cmd/k8sboot/Dockerfile .
docker push ${REPO}:${TAG}
```
