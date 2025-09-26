package main

import (
	"context"
	"net/url"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"gitlab.com/gitlab-org/fleeting/fleeting/provider"
)

// withTestVSphere sets up a simulator and runs the provided test function.
func withTestVSphere(t *testing.T, testFunc func(context.Context, *vSphereDeployment)) {
	model := simulator.VPX()
	defer model.Remove()

	err := model.Run(func(ctx context.Context, c *vim25.Client) error {
		s := c.URL()
		s.User = url.UserPassword("user", "pass")

		client, err := govmomi.NewClient(ctx, s, true)
		if err != nil {
			return err
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

		testFunc(ctx, deployment)
		return nil
	})

	if err != nil {
		t.Fatalf("model.Run() failed: %v", err)
	}
}

func TestVSphereDeployment_Init(t *testing.T) {
	withTestVSphere(t, func(ctx context.Context, deployment *vSphereDeployment) {
		info, err := deployment.Init(ctx, nil, provider.Settings{})
		if err != nil {
			t.Fatalf("Init() failed: %v", err)
		}

		if info.ID != "vSphere" {
			t.Errorf("Expected provider ID to be 'vSphere', got '%s'", info.ID)
		}
	})
}

func TestVSphereDeployment_Increase(t *testing.T) {
	withTestVSphere(t, func(ctx context.Context, deployment *vSphereDeployment) {
		n, err := deployment.Increase(ctx, 1)
		if err != nil {
			t.Fatalf("Increase() failed: %v", err)
		}

		if n != 1 {
			t.Errorf("Expected to increase by 1, but got %d", n)
		}

		// Verify that the VM was created.
		finder := find.NewFinder(deployment.client.Client, true)
		dc, err := finder.Datacenter(ctx, "DC0")
		if err != nil {
			t.Fatalf("Could not find datacenter: %v", err)
		}
		finder.SetDatacenter(dc)

		vms, err := finder.VirtualMachineList(ctx, "/DC0/vm/test-vm-*")
		if err != nil {
			t.Fatalf("Could not list VMs: %v", err)
		}
		if len(vms) != 1 {
			t.Errorf("Expected 1 VM to be created, but found %d", len(vms))
		}
	})
}

func TestVSphereDeployment_Decrease(t *testing.T) {
	withTestVSphere(t, func(ctx context.Context, deployment *vSphereDeployment) {
		// First, increase to have a VM to delete
		_, err := deployment.Increase(ctx, 1)
		if err != nil {
			t.Fatalf("Increase() failed, cannot proceed with Decrease test: %v", err)
		}

		// Find the created VM
		finder := find.NewFinder(deployment.client.Client, true)
		dc, err := finder.Datacenter(ctx, "DC0")
		if err != nil {
			t.Fatalf("Could not find datacenter: %v", err)
		}
		finder.SetDatacenter(dc)

		vms, err := finder.VirtualMachineList(ctx, "/DC0/vm/test-vm-*")
		if err != nil {
			t.Fatalf("Could not list VMs to find one to delete: %v", err)
		}
		if len(vms) == 0 {
			t.Fatal("No VMs found to decrease")
		}

		vmName := vms[0].Name()
		_, err = deployment.Decrease(ctx, []string{vmName})
		if err != nil {
			t.Fatalf("Decrease() failed: %v", err)
		}

		// Verify the VM was deleted
		_, err = finder.VirtualMachineList(ctx, "/DC0/vm/test-vm-*")
		if err == nil {
			t.Fatal("Expected an error when listing deleted VMs, but got nil")
		}
		if _, ok := err.(*find.NotFoundError); !ok {
			t.Fatalf("Expected a NotFoundError, but got: %v", err)
		}
	})
}

func TestVSphereDeployment_ConnectInfo(t *testing.T) {
	withTestVSphere(t, func(ctx context.Context, deployment *vSphereDeployment) {
		// The default simulator VM does not have an IP.
		// We expect an error here.
		_, err := deployment.ConnectInfo(ctx, "DC0_H0_VM0")
		if err == nil {
			t.Fatal("ConnectInfo() should have failed for a VM with no IP, but it did not.")
		}
	})
}

func TestVSphereDeployment_Update(t *testing.T) {
	withTestVSphere(t, func(ctx context.Context, deployment *vSphereDeployment) {
		// First, create a VM to be updated
		_, err := deployment.Increase(ctx, 1)
		if err != nil {
			t.Fatalf("Increase() failed, cannot proceed with Update test: %v", err)
		}

		var updatedInstances []string
		fn := func(instance string, state provider.State) {
			updatedInstances = append(updatedInstances, instance)
		}

		err = deployment.Update(ctx, fn)
		if err != nil {
			t.Fatalf("Update() failed: %v", err)
		}

		if len(updatedInstances) != 1 {
			t.Errorf("Expected to find 1 instance, but got %d", len(updatedInstances))
		}
	})
}
