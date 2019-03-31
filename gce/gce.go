package gce

import (
	"context"
	"fmt"

	"github.com/cenkalti/backoff"
	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	compute "google.golang.org/api/compute/v1"
)

type Client struct {
	svc    *compute.Service
	gSvc   *compute.InstanceGroupsService
	iSvc   *compute.InstancesService
	logger hclog.Logger
}

func NewClient(logger hclog.Logger) (*Client, error) {
	ctx := context.Background()
	computeService, err := compute.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to instantiate Compute Service: %v", err)
	}

	return &Client{
		svc:    computeService,
		logger: logger,
		gSvc:   compute.NewInstanceGroupsService(computeService),
		iSvc:   compute.NewInstancesService(computeService),
	}, nil
}

func (c *Client) ReapableInstances(ctx context.Context, projectID, zone, instanceGroupName string) ([]*compute.InstanceWithNamedPorts, error) {
	result, err := c.gSvc.ListInstances(projectID, zone, instanceGroupName, &compute.InstanceGroupsListInstancesRequest{}).
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}

	var items []*compute.InstanceWithNamedPorts
	for _, i := range result.Items {
		if i.Status == "STOPPING" || i.Status == "STOPPED" || i.Status == "SUSPENDED" || i.Status == "SUSPENDING" || i.Status == "TERMINATED" {
			items = append(items, i)
		}
	}

	return items, nil
}

func (c *Client) WaitForOperationCompletion(ctx context.Context, projectID, zone string, o *compute.Operation) error {
	svc := compute.NewZoneOperationsService(c.svc)
	operation := func() error {
		req := svc.Get(projectID, zone, o.Name)
		o, err := req.Context(ctx).Do()
		if err != nil {
			return backoff.Permanent(err)
		}
		c.logger.Debug("operation status", "status", o.Status)

		if o.Error != nil {
			var oErr error
			for _, err := range o.Error.Errors {
				oErr = multierror.Append(fmt.Errorf("GCE Error %s: %s", err.Code, err.Message))
			}
			return backoff.Permanent(oErr)
		}

		if o.Status == "DONE" {
			return nil
		}

		return fmt.Errorf("Operation status: %s", o.Status)
	}

	return backoff.Retry(operation, backoff.NewExponentialBackOff())
}

func (c *Client) DeleteInstance(ctx context.Context, projectID, zone, instanceName string) error {
	deleteOp, err := c.iSvc.Delete(projectID, zone, instanceName).
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("Failed to delete vm: %v", err)
	}

	return c.WaitForOperationCompletion(ctx, projectID, zone, deleteOp)
}
