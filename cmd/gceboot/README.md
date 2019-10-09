# (Google) Compute Engine boot

Modeled on ec2boot (ec2boot/ec2boot.go)


I (shouldn't but am) having to:
```bash
gcloud auth print-access-token \
| docker login -u oauth2accesstoken --password-stdin https://gcr.io
```

```bash
REPO=gcr.io/bigmachine/gceboot
TAG=$(git rev-parse HEAD)
docker build --tag=${REPO}:${TAG} --file=cmd/gceboot/Dockerfile .
docker push ${REPO}:${TAG}
```