---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "streamnative_resources Data Source - terraform-provider-streamnative"
subcategory: ""
description: |-
  
---

# streamnative_resources (Data Source)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `organization` (String) The organization name
- `resource` (String) The name of StreamNative Cloud resource, should be plural format, valid values are "pools, poolmembers, pulsarclusters, pulsarinstances, pulsarconnections, pulsarenvironments".

### Read-Only

- `id` (String) The ID of this resource.
- `names` (List of String)