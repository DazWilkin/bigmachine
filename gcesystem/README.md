# Google Compute Engine backend

## API Client Library

Uses Google's API Client Library for Compute v1; there is no Golang Cloud Client Library for Compute :-(

## Cloud Resource Manager Library

Needs to be enabled:

```bash
gcloud service enable cloudresourcemanager.googleapis.com --project=${PROJECT}

## Questions

+ Use Container OS and deploy BigMachine bootstrap as a container
+ Would it be better to use a Managed Instance Group rather than create n-instances?

## Docker

Whenever changes are made to gcesystem, the image needs to be rebuilt so that the instances are deployed with the current (!) container

I'm having to:
```bash
gcloud auth print-access-token \
| docker login -u oauth2accesstoken --password-stdin https://gcr.io
```

```bash
PROJECT=
IMG=gcr.io/${PROJECT}/gceboot
TAG=$(git rev-parse HEAD)
docker build --tag=${IMG}:${TAG} --file=cmd/gceboot/Dockerfile .
docker push ${IMG}:${TAG}
```

Hmmm:
```bash
sed --in-place=.bak "s|\"TAG\": \"[0-9a-f]\{40\}\"|\"TAG\": \"${TAG}\"|g" ./.vscode/launch.json
```
**NB** The repetition must be escpaed too `\{40\}`

**NB** We'll reuse `${IMG}` and `${TAG}` in the next section

## Run

Requires a service account with Compute Engine permissions (TBD).

```bash
PROJECT=
ROBOT=bigmachine
FILE=${PWD}/${ROBOT}.json
EMAIL=${ROBOT}@${PROJECT}.iam.gserviceaccount.com

gcloud iam service-accounts create ${ROBOT} --project=$PROJECT

gcloud iam service-accounts keys create ${FILE} \
--iam-account=${EMAIL} \
--project=${PROJECT}

gcloud projects add-iam-policy-binding ${PROJECT} \
--member=serviceAccount:${EMAIL} \
--role=roles/compute.instanceAdmin \
--project=${PROJECT}
```

Service Account must also be able to operate as the Compute Engine instance (using its account)

```bash
PROJECT_NUMBER=$(gcloud projects describe ${PROJECT} --format='value(projectNumber)') && echo ${PROJECT_NUMBER}
gcloud iam service-accounts add-iam-policy-binding ${PROJECT_NUMBER}-compute@developer.gserviceaccount.com \
--member=serviceAccount:${EMAIL} \
--role=roles/iam.serviceAccountUser \
--project=${PROJECT}
```

**NB** 
+ Requires a Google Cloud Platform project ID (`${PROJECT}`) and a zone (`${ZONE}`)
+ `${IMAG}` and `${TAG}` are used to determine the bootstrap image to be used by the GCE instance

```bash
go build && \
GOOGLE_APPLICATION_CREDENTIALS=${FILE} \
PROJECT=${PROJECT} \
ZONE=${ZONE} \
IMG=${IMG} \
TAG=${TAG} \
./bigpi \
  --bigm.system=gce \
  --nmach=2
```
Then:
```bash
gcloud compute instances list --project=${PROJECT}
gcloud compute ssh bigmachine-00 --project=${PROJECT}
```

## Container OS

```bash
docker container ls
CONTAINER ID        IMAGE                                                                COMMAND                  NAMES
316d7ea0f6ff        gcr.io/stackdriver-agents/stackdriver-logging-agent:0.2-1.5.33-1-1   "/entrypoint.sh /usr…"   stackdriver-logging-agent
298c7324ae91        gcr.io/bigmachine/gceboot:02910914c32ee8a3f9debdadc72fa23db918a44a   "/gceboot --log=debu…"   klt-instance-1-lglt

docker logs 298
2019/10/08 20:17:39 Compute Engine backend uses Application Default Credentials. GOOGLE_APPLICATION_CREDENTIALS environment variable is unset
2019/10/08 20:17:41 Compute Engine backend uses Application Default Credentials. GOOGLE_APPLICATION_CREDENTIALS environment variable is unset
```
and:
```
sudo journalctl -u konlet-startup
2019/10/08 20:17:32 Starting Konlet container startup agent
2019/10/08 20:17:32 Downloading credentials for default VM service account from metadata server
2019/10/08 20:17:32 Updating IPtables firewall rules - allowing tcp traffic on all ports
2019/10/08 20:17:32 Launching user container 'gcr.io/bigmachine/gceboot:02910914c32ee8a3f9debdadc72fa23db918a44a'
2019/10/08 20:17:32 Configured container 'instance-1' will be started with name 'klt-instance-1-lglt'.
2019/10/08 20:17:32 Pulling image: 'gcr.io/bigmachine/gceboot:02910914c32ee8a3f9debdadc72fa23db918a44a'
2019/10/08 20:17:36 No containers created by previous runs of Konlet found.
2019/10/08 20:17:37 Found 0 volume mounts in container instance-1 declaration.
2019/10/08 20:17:37 Created a container with name 'klt-instance-1-lglt' and ID: 298c7324ae914b7a48f48bcdc09428051ee51017b3557bcee1d1174785e05019
2019/10/08 20:17:37 Starting a container with ID: 298c7324ae914b7a48f48bcdc09428051ee51017b3557bcee1d1174785e05019
2019/10/08 20:17:39 Saving welcome script to profile.d
Oct 08 20:17:41 instance-1 systemd[1]: konlet-startup.service: Consumed 114ms CPU time
```

## Code

```JSON
{
    "name": "local",
    "type": "go",
    "request": "launch",
    "mode": "auto",
    "program": "cmd/bigpi/bigpi.go",
    "env": {},
    "args": [
        "--bigm.system=local",
        "--nmach=5"
    ]
},
{
    "name": "gce",
    "type": "go",
    "request": "launch",
    "mode": "auto",
    "program": "cmd/bigpi/bigpi.go",
    "env": {
        "GOOGLE_APPLICATION_CREDENTIALS": "...",
        "PROJECT": "bigmachine",
        "ZONE": "us-west1-c",
        "IMG": "gcr.io/bigmachine/gceboot",
        "TAG": "ce2cdb4f74bf050bb08deaf3fb56430c8580d083",
    },
    "args": [
        "--bigm.system=gce",
        "--nmach=2",
    ]
}
```

## References

+ Google Compute Engine [example](https://github.com/googleapis/google-api-go-client/blob/master/examples/compute.go)
+ Operations [link](https://cloud.google.com/compute/docs/api/how-tos/api-requests-responses#handling_api_responses)
+ Compute Engine instance [life-cycle](https://cloud.google.com/compute/docs/instances/instance-life-cycle)
+ Compute Engine Container manifest [link](https://cloud.google.com/deployment-manager/docs/create-container-deployment#create_a_container_manifest)