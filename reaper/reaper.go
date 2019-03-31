package reaper

import (
	"context"
	"strings"
	"time"

	"github.com/endocrimes/gce-terminator/gce"
	hclog "github.com/hashicorp/go-hclog"
)

type Config struct {
	GCPProject        string
	GCPZone           string
	InstanceGroupName string
	PollInterval      *time.Duration
}

type Reaper struct {
	cfg    *Config
	logger hclog.Logger
	gce    *gce.Client
}

func NewReaper(cfg *Config, logger hclog.Logger) *Reaper {
	client, err := gce.NewClient(logger)
	if err != nil {
		// TODO return erros rather than panicing
		panic(err)
	}

	return &Reaper{
		cfg:    cfg,
		logger: logger.Named("reaper"),
		gce:    client,
	}
}

func (r *Reaper) Run(ctx context.Context) error {
	ticker := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:

			if err := r.run(ctx); err != nil {
				r.logger.Error("Reaping failed", "error", err)
			}

			if r.cfg.PollInterval != nil {
				ticker.Reset(*r.cfg.PollInterval)
			} else {
				return nil
			}
		}
	}
}

func (r *Reaper) run(ctx context.Context) error {
	instances, err := r.gce.ReapableInstances(ctx, r.cfg.GCPProject, r.cfg.GCPZone, r.cfg.InstanceGroupName)
	if err != nil {
		return err
	}

	for _, i := range instances {
		components := strings.Split(i.Instance, "/")
		resourceID := components[len(components)-1]
		err := r.gce.DeleteInstance(ctx, r.cfg.GCPProject, r.cfg.GCPZone, resourceID)
		if err != nil {
			r.logger.Error("Error Deleting Instance", "link", i.Instance)
			return err
		}

		r.logger.Info("Deleted Instance", "link", i.Instance)
	}

	return nil
}
