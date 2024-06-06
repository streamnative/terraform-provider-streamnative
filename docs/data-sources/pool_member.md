---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "streamnative_pool_member Data Source - terraform-provider-streamnative"
subcategory: ""
description: |-
  
---

# streamnative_pool_member (Data Source)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The infrastructure pool member name
- `organization` (String) The organization name

### Read-Only

- `id` (String) The ID of this resource.
- `location` (String) The location of the pulsar cluster, supported location https://docs.streamnative.io/docs/cluster#cluster-location
- `pool_name` (String) The infrastructure pool name
- `type` (String) Type of infrastructure pool member, one of aws, gcloud and azure