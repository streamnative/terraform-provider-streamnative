---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "streamnative_pulsar_instance Resource - terraform-provider-streamnative"
subcategory: ""
description: |-
  
---

# streamnative_pulsar_instance (Resource)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `availability_mode` (String) The availability mode, supporting 'zonal' and 'regional'
- `name` (String) The pulsar instance name
- `organization` (String) The organization name
- `pool_name` (String) The infrastructure pool name
- `pool_namespace` (String) The infrastructure pool namespace

### Read-Only

- `id` (String) The ID of this resource.
- `ready` (String) Pulsar instance is ready, it will be set to 'True' after the instance is ready
