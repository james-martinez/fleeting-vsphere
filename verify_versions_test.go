package main

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"gitlab.com/gitlab-org/fleeting/fleeting/provider"
)

func TestVSphereVersions(t *testing.T) {
	versions := []string{"7.0.0.0", "8.0.0.0", "9.0.0.0"}

	for _, version := range versions {
		t.Run(fmt.Sprintf("vSphere-%s", version), func(t *testing.T) {
			verifyVersion(t, version)
		})
	}
}

func verifyVersion(t *testing.T, version string) {
	model := simulator.VPX()
	defer model.Remove()

	model.ServiceContent.About.ApiVersion = version
	model.ServiceContent.About.Version = version

	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		s := c.URL()
		s.User = url.UserPassword("user", "pass")

		// Create client and check if it connects successfully with the given version
		client, err := govmomi.NewClient(ctx, s, true)
		if err != nil {
			return fmt.Errorf("failed to create client for version %s: %v", version, err)
		}

		deployment := &vSphereDeployment{
			client:         client,
			settings:       provider.Settings{},
			Vsphereurl:     s.String(),
			Deploytype:     "clone",
			Datacenter:     "DC0",
			Host:           "DC0_H0",
			Cluster:        "DC0_C0",
			Resourcepool:   "/DC0/host/DC0_C0/Resources",
			Datastore:      "LocalDS_0",
			Contentlibrary: "test-library",
			Network:        "VM Network",
			Template:       "DC0_H0_VM0",
			Folder:         "/DC0/vm/",
			Prefix:         "test-vm",
			Cpu:            "1",
			Memory:         "1024",
		}

		// Verify Init
		_, err = deployment.Init(ctx, nil, provider.Settings{})
		if err != nil {
			return fmt.Errorf("Init() failed for version %s: %v", version, err)
		}

		// Verify Increase (Creating a VM)
		n, err := deployment.Increase(ctx, 1)
		if err != nil {
			return fmt.Errorf("Increase() failed for version %s: %v", version, err)
		}

		if n != 1 {
			return fmt.Errorf("Expected to increase by 1, but got %d", n)
		}

		// Verify that the VM was created.
		finder := find.NewFinder(deployment.client.Client, true)
		dc, err := finder.Datacenter(ctx, "DC0")
		if err != nil {
			return fmt.Errorf("Could not find datacenter: %v", err)
		}
		finder.SetDatacenter(dc)

		vms, err := finder.VirtualMachineList(ctx, "/DC0/vm/test-vm-*")
		if err != nil {
			return fmt.Errorf("Could not list VMs: %v", err)
		}
		if len(vms) != 1 {
			return fmt.Errorf("Expected 1 VM to be created for version %s, but found %d", version, len(vms))
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Test failed for version %s: %v", version, err)
	}
}
