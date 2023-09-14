```
concurrent = 10
check_interval = 0

[[runners]]
  name = "DESKTOP-4M70UBK"
  url = "https://git.fh-muenster.de"
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
    max_instances = 15
    plugin = "fleeting-vsphere"
    [runners.autoscaler.plugin_config]
      vsphereurl = "https://<user>:<pw>@vc-prod.vmware.fh-muenster.de/sdk"
      folder = "/FH-Muenster/vm/Robin/"
      prefix = "prefix-test"
      template = "/FH-Muenster/vm/Robin/ubuntu-parent"
    [runners.autoscaler.connector_config]
      username = "user"
      password = "Passw0rd"
      use_static_credentials = true
      timeout = "1m"
    [[runners.autoscaler.policy]]
      idle_count = 20
      idle_time = "30m"
```
