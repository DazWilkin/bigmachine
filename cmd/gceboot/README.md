# (Google) Compute Engine boot

Modeled on ec2boot (ec2boot/ec2boot.go)

```bash
REPO=gcr.io/bigmachine/gceboot
TAG=$(git rev-parse HEAD)
docker build --tag=${REPO}:${TAG} --file=cmd/gceboot/Dockerfile .
```