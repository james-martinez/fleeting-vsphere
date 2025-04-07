package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/vcenter"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"gitlab.com/gitlab-org/fleeting/fleeting/plugin"
	"gitlab.com/gitlab-org/fleeting/fleeting/provider"
)

var _ provider.InstanceGroup = &vSphereDeployment{}

type vSphereDeployment struct {
	client   *govmomi.Client
	settings provider.Settings

	Vsphereurl     string
	Datacenter     string
	Cluster        string
	Resourcepool   string
	Contentlibrary string
	Datastore      string
	Ovftemplate    string
	Template       string
	Folder         string
	Prefix         string
	Cpu            string
	Memory         string
	Portgroup      string
}

// Add a method in vSphereDeployment to deploy an OVF from the content library
func (k *vSphereDeployment) DeployOVFFromLibrary(ctx context.Context, libraryName, templateName, vmName string) error {
	c := k.client.Client
	m := vcenter.NewManager(c)

	// Find the library item
	item, err := library.FindItem(ctx, c, libraryName, templateName)
	if err != nil {
		return fmt.Errorf("unable to find library item: %v", err)
	}

	// Setup deployment target
	finder := find.NewFinder(c, false)
	rp, err := finder.DefaultResourcePool(ctx)
	if err != nil {
		return fmt.Errorf("unable to find resource pool: %v", err)
	}
	folder, err := finder.Folder(ctx, k.Folder)
	if err != nil {
		return fmt.Errorf("unable to find folder: %v", err)
	}

	// Setup deployment spec
	deploy := vcenter.Deploy{
		DeploymentSpec: vcenter.DeploymentSpec{
			Name:               vmName,
			AcceptAllEULA:      true,
			DefaultDatastoreID: "",  // Set your Datastore ID if required
			NetworkMappings:    nil, // Set your network mappings
			StorageProfileID:   "",  // Set your storage profile if required
		},
		Target: vcenter.Target{
			ResourcePoolID: rp.Reference().Value,
			FolderID:       folder.Reference().Value,
		},
	}

	// Deploy the library item
	ref, err := m.DeployLibraryItem(ctx, item.ID, deploy)
	if err != nil {
		return fmt.Errorf("deployment failed: %v", err)
	}

	fmt.Printf("Deployed VM: %s\n", ref.Value)
	return nil
}

func (k *vSphereDeployment) Init(ctx context.Context, logger hclog.Logger, settings provider.Settings) (provider.ProviderInfo, error) {
	if !settings.UseStaticCredentials {
		return provider.ProviderInfo{}, fmt.Errorf("this plugin cannot provision credentials")
	}
	if k.Vsphereurl == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide vsphereurl in plug_config")
	}
	if k.Template == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide template in plug_config")
	}
	if k.Folder == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide folder in plug_config")
	}
	if k.Prefix == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide prefix in plug_config")
	}
	url, err := url.Parse(k.Vsphereurl)
	if err != nil {
		return provider.ProviderInfo{}, err
	}

	k.client, err = govmomi.NewClient(ctx, url, true)
	if err != nil {
		return provider.ProviderInfo{}, err
	}
	version := os.Getenv("VERSION")
	if version == "" {
		version = "0.1.0"
	}

	buildInfo := os.Getenv("BUILD_INFO")
	if buildInfo == "" {
		buildInfo = "HEAD"
	}

	k.settings = settings
	return provider.ProviderInfo{
		ID:        "vSphere",
		MaxSize:   50,
		Version:   version,
		BuildInfo: buildInfo,
	}, nil
}

func (k *vSphereDeployment) Update(ctx context.Context, fn func(instance string, state provider.State)) error {
	finder := find.NewFinder(k.client.Client, false)

	folder, err := finder.Folder(ctx, k.Folder)

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

			if strings.HasPrefix(vmInfo.Name, k.Prefix) {
				state := determineState(vmInfo)
				fn(vmInfo.Name, state)
			}
		}
	}
	return nil
}

func (k *vSphereDeployment) Increase(ctx context.Context, n int) (int, error) {

	srcPath := k.Template
	destPath := k.Folder

	finder := find.NewFinder(k.client.Client, false)
	srcVM, _ := finder.VirtualMachine(ctx, srcPath)

	destFolder, _ := finder.Folder(ctx, path.Dir(destPath))

	destFolderRef := destFolder.Reference()

	numClones := n // number of clones
	var wg sync.WaitGroup

	for i := 1; i <= numClones; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cloneVM(ctx, k.client, srcVM, destFolderRef, k.Prefix, finder, i)
		}(i)
	}

	wg.Wait()

	return n, nil
}

func (k *vSphereDeployment) Decrease(ctx context.Context, instances []string) ([]string, error) {

	finder := find.NewFinder(k.client.Client, true)

	if err := deleteVMs(ctx, k.client, finder, k.Folder, instances); err != nil {
		fmt.Printf("Error deleting VMs: %v\n", err)
	} else {
	}
	return instances, nil
}

func (k *vSphereDeployment) ConnectInfo(ctx context.Context, instance string) (provider.ConnectInfo, error) {
	finder := find.NewFinder(k.client.Client, true)

	var name = k.Folder + instance
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
		ConnectorConfig: k.settings.ConnectorConfig,
		ID:              instance,
		InternalAddr:    ip,
		Expires:         &expires,
	}, nil
}

func (k *vSphereDeployment) Shutdown(ctx context.Context) error {
	return nil
}

func cloneVM(ctx context.Context, client *govmomi.Client, srcVM *object.VirtualMachine, destFolderRef types.ManagedObjectReference, prefix string, finder *find.Finder, cloneNumber int) {
	manager := library.NewManager(rest.Client)

	uuid := uuid.New()
	// req := types.InstantClone_Task{
	// 	This: srcVM.Reference(),
	// 	Spec: types.VirtualMachineInstantCloneSpec{
	// 		Name: fmt.Sprintf("%s-%s", prefix, uuid),
	// 		Location: types.VirtualMachineRelocateSpec{
	// 			Folder: &destFolderRef,
	// 		},
	// 	},
	// }

	//res, _ := methods.InstantClone_Task(ctx, client.Client, &req)

	// task := object.NewTask(client.Client, res.Returnval)
	// _ = task.Wait(ctx)
	deploySpec := vcenter.Deploy{
		DeploymentSpec: vcenter.DeploymentSpec{
			Name:               "vm-name",
			DefaultDatastoreID: "datastore-id",
			AcceptAllEULA:      true,
		},
		Target: vcenter.Target{
			ResourcePoolID: "resource-pool-id",
			HostID:         "host-id",   // Optional
			FolderID:       "folder-id", // Optional
		},
	}

	item := library.Item{
		ID: "library-item-id",
	}

	deployment, err := manager.DeployLibraryItem(ctx, item, deploySpec)
	if err != nil {
		log.Fatal(err)
	}
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
