---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "streamnative_cloud_environment Data Source - terraform-provider-streamnative"
subcategory: ""
description: |-
  
---

# streamnative_cloud_environment (Data Source)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Name of the cloud environment
- `organization` (String) The organization name

### Read-Only

- `cloud_connection_name` (String) Name of the cloud connection
- `id` (String) The ID of this resource.
- `network` (List of Object) (see [below for nested schema](#nestedatt--network))
- `region` (String)

<a id="nestedatt--network"></a>
### Nested Schema for `network`

Read-Only:

- `cidr` (String)
- `id` (String)


