package gcesystem

import (
	"cloud.google.com/go/compute/metadata"
)

func onGCE() bool {
	return metadata.OnGCE()
}
