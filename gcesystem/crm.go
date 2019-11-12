package gcesystem

import (
	"context"
	"log"

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
)

// ProjectNumber returns a project number for a given project ID
func ProjectNumber(ctx context.Context, ID string) (int64, error) {
	service, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return 0, err
	}
	p, err := service.Projects.Get(ID).Context(ctx).Do()
	if err != nil {
		return 0, err
	}
	log.Printf("[ProjectNumber] %s-->%d", ID, p.ProjectNumber)
	return p.ProjectNumber, nil
}
