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

Ensure the environment variable `TAG` is set when running e.g. `cmd/bigpi/bigpi.go` or debugging (`launch.json`) too

**NB** If the Dockerfile `USER 999` is included, the container is unable to run privileged ports e.g. 443