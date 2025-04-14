fork of https://git.fh-muenster.de/robin/fleeting-vsphere

# Planned Features

- Integrare library.deploy from govc
- Integrate vm.clone using VM Template from govc
- Integrate vm.instantclone from govc

```
concurrent = 10
check_interval = 0

[[runners]]
  name = "DESKTOP-4M70UBK"
  url = "https://gitlab.tld"
  token = "<token>"
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
    plugin = "fleeting-vsphere"
    [runners.autoscaler.plugin_config]
      vsphereurl = "https://<user>:<pw>@vsphere.tld/sdk"
      deploytype =  "contentlibrary"
      datacenter = ""
      host = ""
      cluster = ""
      resourcepool = ""
      datastore = ""
      contentlibrary = ""
      network = ""
      folder = "/FH-Muenster/vm/Robin/"
      prefix = "prefix-test"
      template = "/FH-Muenster/vm/Robin/ubuntu-parent"
      cpu = "2"
      memory = "4096"
    [runners.autoscaler.connector_config]
      username = "user"
      password = "Passw0rd"
      use_static_credentials = true
      timeout = "1m"
    [[runners.autoscaler.policy]]
      idle_count = 20
      idle_time = "30m"
```
