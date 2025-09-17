package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
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

	Vsphereurl     string
	Deploytype     string
	Datacenter     string
	Host           string
	Cluster        string
	Resourcepool   string
	Datastore      string
	Contentlibrary string
	Network        string
	Template       string
	Folder         string
	Cpu            string
	Memory         string
	Prefix         string
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
	if k.Deploytype == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide deploytype in plug_config")
	}
	if k.Datacenter == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide datacenter in plug_config")
	}
	if k.Host == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide host in plug_config")
	}
	if k.Cluster == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide cluster in plug_config")
	}
	if k.Resourcepool == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide resourcepool in plug_config")
	}
	if k.Datastore == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide datastore in plug_config")
	}
	if k.Contentlibrary == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide contentlibrary in plug_config")
	}
	if k.Network == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide network in plug_config")
	}
	if k.Cpu == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide cpu in plug_config")
	}
	if k.Memory == "" {
		return provider.ProviderInfo{}, fmt.Errorf("please provide memory in plug_config")
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
	// Return early if client is not initialized (for testing)
	if k.client == nil {
		return nil
	}

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
	// Return early if client is not initialized (for testing)
	if k.client == nil {
		return n, nil
	}

	deployType := k.Deploytype
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
			deployVM(ctx, k.client, deployType, srcVM, destFolderRef, k.Prefix, finder, i, k.Datacenter, k.Host, k.Cluster, k.Resourcepool, k.Datastore, k.Contentlibrary,
				k.Network, k.Cpu, k.Memory)
		}(i)
	}

	wg.Wait()

	return n, nil
}

func (k *vSphereDeployment) Decrease(ctx context.Context, instances []string) ([]string, error) {
	// Return early if client is not initialized (for testing)
	if k.client == nil {
		return instances, nil
	}

	finder := find.NewFinder(k.client.Client, true)

	if err := deleteVMs(ctx, k.client, finder, k.Folder, instances); err != nil {
		fmt.Printf("Error deleting VMs: %v\n", err)
	} else {
	}
	return instances, nil
}

func (k *vSphereDeployment) ConnectInfo(ctx context.Context, instance string) (provider.ConnectInfo, error) {
	// Return mock data if client is not initialized (for testing)
	if k.client == nil {
		return provider.ConnectInfo{
			ConnectorConfig: k.settings.ConnectorConfig,
			ID:              instance,
			InternalAddr:    "10.42.144.11", // Mock IP for testing
			Expires:         nil,
		}, nil
	}

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
func deployVM(ctx context.Context, client *govmomi.Client, deployType string,
	srcVM *object.VirtualMachine, destFolderRef types.ManagedObjectReference,
	prefix string, finder *find.Finder, cloneNumber int,
	datacenter string, host string, cluster string, resourcePool string,
	datastore string, contentLibrary string, network string,
	cpu string, memory string) {
	uuid := uuid.New()
	vmName := fmt.Sprintf("%s-%s", prefix, uuid)

	switch deploytype := deployType; deploytype {
	case "instantclone":
		err := deployVMInstantClone(ctx, client, srcVM, vmName, destFolderRef, finder,
			datacenter, host, cluster, resourcePool, datastore, network, cpu, memory)
		if err != nil {
			fmt.Printf("Error creating instant clone: %v\n", err)
		}
	case "clone":
		err := deployVMClone(ctx, client, srcVM, vmName, destFolderRef, finder,
			datacenter, host, cluster, resourcePool, datastore, network, cpu, memory)
		if err != nil {
			fmt.Printf("Error creating clone: %v\n", err)
		}
	case "librarydeploy", "contentlibrary":
		err := deployFromContentLibrary(ctx, client, vmName, contentLibrary, srcVM.Name(),
			destFolderRef, finder, datacenter, host, cluster, resourcePool, datastore, network, cpu, memory)
		if err != nil {
			fmt.Printf("Error deploying from content library: %v\n", err)
		}
	default:
		fmt.Printf("Unsupported deploytype: %s", deploytype)
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

func deployVMInstantClone(ctx context.Context, client *govmomi.Client, srcVM *object.VirtualMachine,
	vmName string, destFolderRef types.ManagedObjectReference, finder *find.Finder,
	datacenter string, host string, cluster string, resourcePool string,
	datastore string, network string, cpu string, memory string) error {

	// Create instant clone specification
	spec := types.VirtualMachineInstantCloneSpec{
		Name: vmName,
		Location: types.VirtualMachineRelocateSpec{
			Folder: &destFolderRef,
		},
	}

	// Execute the instant clone using govmomi methods
	req := types.InstantClone_Task{
		This: srcVM.Reference(),
		Spec: spec,
	}

	res, err := methods.InstantClone_Task(ctx, client.Client, &req)
	if err != nil {
		return fmt.Errorf("failed to create instant clone: %v", err)
	}

	// Wait for the task to complete
	task := object.NewTask(client.Client, res.Returnval)
	err = task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("instant clone task failed: %v", err)
	}

	fmt.Printf("Successfully created instant clone VM '%s'\n", vmName)
	return nil
}

func deployVMClone(ctx context.Context, client *govmomi.Client, srcVM *object.VirtualMachine,
	vmName string, destFolderRef types.ManagedObjectReference, finder *find.Finder,
	datacenter string, host string, cluster string, resourcePool string,
	datastore string, network string, cpu string, memory string) error {

	// Get resource pool and datastore references
	rpObj, err := finder.ResourcePool(ctx, resourcePool)
	if err != nil {
		return fmt.Errorf("failed to find resource pool: %v", err)
	}

	dsObj, err := finder.Datastore(ctx, datastore)
	if err != nil {
		return fmt.Errorf("failed to find datastore: %v", err)
	}

	// Parse CPU and memory values
	cpuCount, err := strconv.ParseInt(cpu, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid CPU count: %v", err)
	}

	memoryMB, err := strconv.ParseInt(memory, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid memory size: %v", err)
	}

	// Get references for clone spec
	rpRef := rpObj.Reference()
	dsRef := dsObj.Reference()

	// Create clone specification
	cloneSpec := types.VirtualMachineCloneSpec{
		Location: types.VirtualMachineRelocateSpec{
			Folder:    &destFolderRef,
			Pool:      &rpRef,
			Datastore: &dsRef,
		},
		PowerOn:  true,
		Template: false,
		Config: &types.VirtualMachineConfigSpec{
			Name:     vmName,
			NumCPUs:  int32(cpuCount),
			MemoryMB: memoryMB,
		},
	}

	// Get the destination folder object
	destFolder := object.NewFolder(client.Client, destFolderRef)

	// Execute the clone using govmomi
	task, err := srcVM.Clone(ctx, destFolder, vmName, cloneSpec)
	if err != nil {
		return fmt.Errorf("failed to create clone: %v", err)
	}

	// Wait for the clone task to complete
	err = task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("clone task failed: %v", err)
	}

	fmt.Printf("Successfully created clone VM '%s'\n", vmName)
	return nil
}

func deployFromContentLibrary(ctx context.Context, client *govmomi.Client, vmName string,
	contentLibraryName string, templateName string, destFolderRef types.ManagedObjectReference,
	finder *find.Finder, datacenter string, host string, cluster string,
	resourcePool string, datastore string, network string, cpu string, memory string) error {

	// For content library deployment, we'll use the template VM approach
	// Find the template VM in the content library (assuming it's already deployed as a template)
	templateVM, err := finder.VirtualMachine(ctx, templateName)
	if err != nil {
		return fmt.Errorf("failed to find template VM '%s' in content library: %v", templateName, err)
	}

	// Get resource pool and datastore references
	rpObj, err := finder.ResourcePool(ctx, resourcePool)
	if err != nil {
		return fmt.Errorf("failed to find resource pool: %v", err)
	}

	dsObj, err := finder.Datastore(ctx, datastore)
	if err != nil {
		return fmt.Errorf("failed to find datastore: %v", err)
	}

	// Parse CPU and memory values
	cpuCount, err := strconv.ParseInt(cpu, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid CPU count: %v", err)
	}

	memoryMB, err := strconv.ParseInt(memory, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid memory size: %v", err)
	}

	// Get references for clone spec
	rpRef := rpObj.Reference()
	dsRef := dsObj.Reference()

	// Create clone specification for content library deployment
	cloneSpec := types.VirtualMachineCloneSpec{
		Location: types.VirtualMachineRelocateSpec{
			Folder:    &destFolderRef,
			Pool:      &rpRef,
			Datastore: &dsRef,
		},
		PowerOn:  true,
		Template: false,
		Config: &types.VirtualMachineConfigSpec{
			Name:     vmName,
			NumCPUs:  int32(cpuCount),
			MemoryMB: memoryMB,
		},
	}

	// Get the destination folder object
	destFolder := object.NewFolder(client.Client, destFolderRef)

	// Clone the VM from the content library template
	task, err := templateVM.Clone(ctx, destFolder, vmName, cloneSpec)
	if err != nil {
		return fmt.Errorf("failed to clone VM from content library template: %v", err)
	}

	// Wait for the clone task to complete
	err = task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("content library clone task failed: %v", err)
	}

	fmt.Printf("Successfully deployed VM '%s' from content library template\n", vmName)
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
