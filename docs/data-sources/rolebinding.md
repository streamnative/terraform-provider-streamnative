---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "streamnative_rolebinding Data Source - terraform-provider-streamnative"
subcategory: ""
description: |-
  
---

# streamnative_rolebinding (Data Source)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The name of rolebinding
- `organization` (String) The organization name

### Read-Only

- `cluster_role_name` (String) The predefined role name
- `id` (String) The ID of this resource.
- `ready` (Boolean) The RoleBinding is ready, it will be set to 'True' after the cluster is ready
- `service_account_names` (List of String) The list of service accounts that are role binding names
