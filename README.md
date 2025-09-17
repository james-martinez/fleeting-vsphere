# Fleeting vSphere Plugin

Fork of https://git.fh-muenster.de/robin/fleeting-vsphere

A GitLab Runner autoscaler plugin for VMware vSphere that provides flexible VM deployment options using govmomi APIs.

## Features ✅

- ✅ **Content Library Deployment** - Deploy VMs from vSphere content library templates
- ✅ **Traditional VM Cloning** - Clone VMs from existing VM templates with full customization
- ✅ **Instant Cloning** - Rapid VM provisioning with minimal storage overhead
- ✅ **Hardware Customization** - Configure CPU, memory, and network settings during deployment
- ✅ **Resource Management** - Automatic VM lifecycle management (create, monitor, destroy)

## Deployment Methods

### 1. Content Library Deployment (`librarydeploy` or `contentlibrary`)
Deploys VMs from standardized templates stored in vSphere content libraries.

> **Note:** Both `librarydeploy` and `contentlibrary` are supported for backward compatibility.

**Benefits:**
- Centralized template management
- Version control for VM templates
- Consistent deployments across environments

### 2. Traditional VM Clone (`clone`)
Creates full clones from existing VM templates with complete customization.

**Benefits:**
- Full hardware customization
- Independent storage allocation
- Works with any VM state (powered on/off)

### 3. Instant Clone (`instantclone`)
Rapid VM provisioning using vSphere instant clone technology.

**Benefits:**
- Fastest deployment method
- Minimal storage overhead (linked clones)
- Requires parent VM to be powered on

## Configuration

### GitLab Runner Configuration Example

```toml
concurrent = 10
check_interval = 0

[[runners]]
  name = "vsphere-runner"
  url = "https://gitlab.example.com"
  token = "<your-token>"
  executor = "docker-autoscaler"
  limit = 40
  [runners.docker]
    image = "busybox:latest"
  [runners.cache]
    MaxUploadedArchiveSize = 0
  [runners.autoscaler]
    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 5
    plugin = "ghcr.io/james-martinez/fleeting-vsphere:latest"  # OCI registry image
    [runners.autoscaler.plugin_config]
      vsphereurl = "https://<user>:<password>@vcenter.example.com/sdk"
      deploytype = "librarydeploy"  # Options: "librarydeploy"/"contentlibrary", "clone", "instantclone"
      datacenter = "Datacenter1"
      host = "esxi-host.example.com"
      cluster = "Cluster1"
      resourcepool = "ResourcePool1"
      datastore = "datastore1"
      contentlibrary = "GitLab-Templates"  # Required for librarydeploy
      network = "VM Network"
      folder = "/Datacenter1/vm/GitLab-Runners/"
      prefix = "gitlab-runner"
      template = "ubuntu-20.04-template"  # Template name in content library or VM path
      cpu = "2"
      memory = "4096"
    [runners.autoscaler.connector_config]
      username = "gitlab"
      password = "SecurePassword123"
      use_static_credentials = true
      timeout = "1m"
    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "10m"
```

### Configuration Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `vsphereurl` | ✅ | vCenter Server URL with credentials | `https://user:pass@vcenter.com/sdk` |
| `deploytype` | ✅ | Deployment method | `librarydeploy`/`contentlibrary`, `clone`, `instantclone` |
| `datacenter` | ✅ | vSphere datacenter name | `Datacenter1` |
| `host` | ✅ | ESXi host for VM placement | `esxi-host.example.com` |
| `cluster` | ✅ | vSphere cluster name | `Cluster1` |
| `resourcepool` | ✅ | Resource pool for VMs | `ResourcePool1` |
| `datastore` | ✅ | Datastore for VM storage | `datastore1` |
| `contentlibrary` | ✅* | Content library name (*required for librarydeploy) | `GitLab-Templates` |
| `network` | ✅ | Network for VM connectivity | `VM Network` |
| `folder` | ✅ | VM folder path | `/Datacenter1/vm/GitLab-Runners/` |
| `prefix` | ✅ | VM name prefix | `gitlab-runner` |
| `template` | ✅ | Template name or VM path | `ubuntu-20.04-template` |
| `cpu` | ✅ | Number of CPU cores | `2` |
| `memory` | ✅ | Memory in MB | `4096` |

## Deployment Type Comparison

| Feature | Instant Clone | Traditional Clone | Content Library |
|---------|---------------|-------------------|-----------------|
| **Speed** | Fastest | Moderate | Moderate |
| **Storage** | Minimal (linked) | Full copy | Full copy |
| **Parent VM State** | Must be powered on | Any state | Any state |
| **Customization** | Limited | Full | Full |
| **Use Case** | Dev/Test environments | Production workloads | Standardized deployments |

## Building

```bash
go build -o fleeting-vsphere .
```

## Installation

### OCI Registry Distribution

The fleeting-vsphere plugin is available as a container image from GitHub Container Registry:

```bash
# Pull the latest version
docker pull ghcr.io/james-martinez/fleeting-vsphere:latest

# Pull a specific version
docker pull ghcr.io/james-martinez/fleeting-vsphere:v1.0.0
```

### GitLab Runner Configuration with OCI Image

Configure GitLab Runner to use the containerized plugin directly from the registry:

```toml
concurrent = 10
check_interval = 0

[[runners]]
  name = "vsphere-runner"
  url = "https://gitlab.example.com"
  token = "<your-token>"
  executor = "docker-autoscaler"
  limit = 40
  [runners.docker]
    image = "busybox:latest"
  [runners.autoscaler]
    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 5
    # Use the OCI image directly from registry
    plugin = "ghcr.io/james-martinez/fleeting-vsphere:latest"
    [runners.autoscaler.plugin_config]
      vsphereurl = "https://<user>:<password>@vcenter.example.com/sdk"
      deploytype = "librarydeploy"
      datacenter = "Datacenter1"
      host = "esxi-host.example.com"
      cluster = "Cluster1"
      resourcepool = "ResourcePool1"
      datastore = "datastore1"
      contentlibrary = "GitLab-Templates"
      network = "VM Network"
      folder = "/Datacenter1/vm/GitLab-Runners/"
      prefix = "gitlab-runner"
      template = "ubuntu-20.04-template"
      cpu = "2"
      memory = "4096"
    [runners.autoscaler.connector_config]
      username = "gitlab"
      password = "SecurePassword123"
      use_static_credentials = true
      timeout = "1m"
    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "10m"
```

### GitLab Runner Registration

When registering a GitLab Runner with fleeting support, GitLab Runner will automatically pull and manage the container:

```bash
# Register GitLab Runner with fleeting-vsphere plugin
gitlab-runner register \
  --url "https://gitlab.example.com" \
  --token "your-registration-token" \
  --executor "docker-autoscaler" \
  --docker-image "busybox:latest" \
  --description "vSphere Fleeting Runner"

# The plugin configuration is handled in config.toml as shown above
```

### Available Tags

- `latest` - Latest stable release
- `v1.0.0`, `v1.1.0`, etc. - Specific version releases
- `main` - Latest development build
- `develop` - Development branch builds

### Building Locally (Optional)

If you prefer to build the image locally instead of using the registry:

```bash
# Build the image
docker build -t fleeting-vsphere:latest .

# Or build with specific version
docker build -t fleeting-vsphere:v1.0.0 --build-arg VERSION=v1.0.0 .
```


## Testing

```bash
go test -v
```

## Requirements

- Go 1.19+
- VMware vSphere 6.7+
- GitLab Runner 15.0+
- Network connectivity to vCenter Server

## Troubleshooting

### Common Issues

1. **Authentication Failures**
   - Verify vCenter credentials in `vsphereurl`
   - Ensure user has required permissions

2. **Template Not Found**
   - For `librarydeploy`: Verify template exists in specified content library
   - For `clone`/`instantclone`: Verify VM template path is correct

3. **Resource Allocation Errors**
   - Check resource pool has sufficient CPU/memory
   - Verify datastore has adequate free space

4. **Network Connectivity**
   - Ensure specified network exists and is accessible
   - Verify VM can obtain IP address from network

### Logging

Enable debug logging in GitLab Runner configuration:
```toml
log_level = "debug"
```

## License

This project is licensed under the MIT License.
