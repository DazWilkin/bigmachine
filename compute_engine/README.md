# Google Compute Engine backend

## API Client Library

Uses Google's API Client Library for Compute v1; there is no Golang Cloud Client Library for Compute :-(

## Run

Requires a service account with Compute Engine permissions (TBD).

Requires a Google Cloud Platform project ID (`${PROJECT}`) and a zone (`${ZONE}`)
```bash
go build && \
GOOGLE_APPLICATION_CREDENTIALS=${CREDENTIALS} \
PROJECT=${PROJECT} \
ZONE=${ZONE} \
./bigpi \
  -bigm.system=gce
```