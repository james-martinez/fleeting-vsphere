package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"gitlab.com/gitlab-org/fleeting/fleeting/plugin"
	"gitlab.com/gitlab-org/fleeting/fleeting/provider"
)

var _ provider.InstanceGroup = &vSphereDeployment{}

type vSphereDeployment struct {
	client   *govmomi.Client
	settings provider.Settings

	Vsphereurl string
}

func (k *vSphereDeployment) Init(ctx context.Context, logger hclog.Logger, settings provider.Settings) (provider.ProviderInfo, error) {
	if !settings.UseStaticCredentials {
		return provider.ProviderInfo{}, fmt.Errorf("this plugin cannot provision credentials")
	}
	if k.Vsphereurl == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide vsphereurl in plug_config")
	}
	url, err := url.Parse(k.Vsphereurl)
	if err != nil {
		return provider.ProviderInfo{}, err
	}

	k.client, err = govmomi.NewClient(ctx, url, true)
	if err != nil {
		return provider.ProviderInfo{}, err
	}

	k.settings = settings
	return provider.ProviderInfo{
		ID:        "vSphere",
		MaxSize:   10,
		Version:   "0.1.0",
		BuildInfo: "HEAD",
	}, nil
}

func (k *vSphereDeployment) Update(ctx context.Context, fn func(instance string, state provider.State)) error {
	finder := find.NewFinder(k.client.Client, false)

	folder, err := finder.Folder(ctx, "/FH-Muenster/vm/Robin/")

	if err != nil {
		return err
	}

	var folderRef mo.Folder
	err = folder.Properties(ctx, folder.Reference(), []string{"childEntity"}, &folderRef)
	if err != nil {
		return err
	}

	for _, ref := range folderRef.ChildEntity {
		if ref.Type == "VirtualMachine" {
			vm := object.NewVirtualMachine(k.client.Client, ref.Reference())
			var vmInfo mo.VirtualMachine
			err = vm.Properties(ctx, vm.Reference(), []string{"name", "runtime", "guest"}, &vmInfo)
			if err != nil {
				return err
			}

			if strings.HasPrefix(vmInfo.Name, "ubuntu-child") {
				state := determineState(vmInfo)
				fn(vmInfo.Name, state)
			}
		}
	}
	return nil
}

func (k *vSphereDeployment) Increase(ctx context.Context, n int) (int, error) {

	srcPath := "/FH-Muenster/vm/Robin/ubuntu-parent"
	destPath := "/FH-Muenster/vm/Robin/ubuntu-child"

	finder := find.NewFinder(k.client.Client, false)
	srcVM, _ := finder.VirtualMachine(ctx, srcPath)

	destFolder, _ := finder.Folder(ctx, path.Dir(destPath))

	destFolderRef := destFolder.Reference()

	numClones := n // number of clones
	var wg sync.WaitGroup

	start := time.Now()

	for i := 1; i <= numClones; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cloneVM(ctx, k.client, srcVM, destFolderRef, destPath, finder)
		}(i)
	}

	wg.Wait()

	elapsed := time.Since(start)
	fmt.Println("The whole process took ", elapsed)
	return n, nil
}

func (k *vSphereDeployment) Decrease(ctx context.Context, instances []string) ([]string, error) {

	finder := find.NewFinder(k.client.Client, true)

	if err := deleteVMs(ctx, k.client, finder, "/FH-Muenster/vm/Robin/", instances); err != nil {
		fmt.Printf("Error deleting VMs: %v\n", err)
	} else {
		fmt.Println("Successfully deleted VMs")
	}
	return instances, nil
}

func (k *vSphereDeployment) ConnectInfo(ctx context.Context, instance string) (provider.ConnectInfo, error) {
	finder := find.NewFinder(k.client.Client, true)

	var name = "/FH-Muenster/vm/" + instance
	vm, err := finder.VirtualMachine(ctx, name)
	if err != nil {
		return provider.ConnectInfo{}, err
	}

	var vmInfo mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"guest.net"}, &vmInfo)
	if err != nil {
		return provider.ConnectInfo{}, err
	}

	var ip string
	nics := vmInfo.Guest.Net
	for _, nic := range nics {
		if ip != "" {
			break
		}

		if mac := nic.MacAddress; mac == "" {
			continue
		}
		if nic.IpConfig == nil {
			continue
		}

		for _, vmIP := range nic.IpAddress {
			if net.ParseIP(vmIP).To4() == nil {
				continue
			}

			ip = vmIP
			break
		}
	}

	if ip == "" {
		return provider.ConnectInfo{}, fmt.Errorf("could not find an IPv4 address for VM: %s", instance)
	}

	expires := time.Now().Add(5 * time.Minute)

	return provider.ConnectInfo{
		ID:           instance,
		InternalAddr: ip,
		Expires:      &expires,
	}, nil
}

func cloneVM(ctx context.Context, client *govmomi.Client, srcVM *object.VirtualMachine, destFolderRef types.ManagedObjectReference, destPath string, finder *find.Finder) {
	uuid := uuid.New()
	req := types.InstantClone_Task{
		This: srcVM.Reference(),
		Spec: types.VirtualMachineInstantCloneSpec{
			Name: fmt.Sprintf("%s-%s", path.Base(destPath), uuid),
			Location: types.VirtualMachineRelocateSpec{
				Folder: &destFolderRef,
			},
		},
	}

	start := time.Now()
	fmt.Printf("Cloning VM %d...\n", cloneNumber)
	res, _ := methods.InstantClone_Task(ctx, client.Client, &req)

	task := object.NewTask(client.Client, res.Returnval)
	_ = task.Wait(ctx)

	elapsed := time.Since(start)
	fmt.Printf("Clone %d succeeded. Took %s\n", cloneNumber, elapsed)

	start = time.Now()
	fmt.Printf("Waiting for IP for VM %d...\n", cloneNumber)

	clonedVM, _ := finder.VirtualMachine(ctx, fmt.Sprintf("%s-%d", destPath, cloneNumber))

	var ip string
	var vmInfo mo.VirtualMachine

	for ip == "" {
		_ = clonedVM.Properties(ctx, clonedVM.Reference(), []string{"guest.net"}, &vmInfo)

		nics := vmInfo.Guest.Net
		for _, nic := range nics {
			if ip != "" {
				break
			}

			if mac := nic.MacAddress; mac == "" {
				continue
			}
			if nic.IpConfig == nil {
				continue
			}

			for _, vmIP := range nic.IpAddress {
				if net.ParseIP(vmIP).To4() == nil {
					continue
				}

				ip = vmIP
				break
			}
		}

		// No IP found on the VM yet
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("VM %d IP: %s\n", cloneNumber, ip)

	elapsed = time.Since(start)
	fmt.Printf("Waiting for IP for VM %d took %s\n", cloneNumber, elapsed)
}

func deleteVMs(ctx context.Context, client *govmomi.Client, finder *find.Finder, folder string, vmNames []string) error {
	for _, vmName := range vmNames {
		var name = folder + vmName
		vm, err := finder.VirtualMachine(ctx, name)
		if err != nil {
			return fmt.Errorf("error finding VM %s: %v", name, err)
		}

		task, err := vm.PowerOff(ctx)
		if err != nil {
			return fmt.Errorf("error powering off VM %s: %v", vmName, err)
		}

		if err := task.Wait(ctx); err != nil {
			return fmt.Errorf("error waiting for power off task for VM %s: %v", vmName, err)
		}

		task, err = vm.Destroy(ctx)
		if err != nil {
			return fmt.Errorf("error destroying VM %s: %v", vmName, err)
		}

		if err := task.Wait(ctx); err != nil {
			return fmt.Errorf("error waiting for destroy task for VM %s: %v", vmName, err)
		}
	}

	return nil
}

func determineState(vm mo.VirtualMachine) provider.State {
	if vm.Runtime.PowerState != "poweredOn" {
		return provider.StateDeleting
	}

	var ip string
	for _, nic := range vm.Guest.Net {
		if ip != "" {
			break
		}

		if mac := nic.MacAddress; mac == "" {
			continue
		}
		if nic.IpConfig == nil {
			continue
		}

		for _, vmIP := range nic.IpAddress {
			if net.ParseIP(vmIP).To4() == nil {
				continue
			}

			ip = vmIP
			break
		}
	}

	if ip == "" {
		return provider.StateCreating
	}

	return provider.StateRunning
}

func main() {
	plugin.Serve(&vSphereDeployment{})
}
