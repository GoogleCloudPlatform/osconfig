# Prototype to deploy OSConfig Guest Policies in Multiple GCP Projects

This guide describes how to use [Terraform](https://www.terraform.io/) to deploy an OSConfig Guest Policy in multiple GCP Projects.

It proceeds through the following stages:

*  Determine the list of GCP Projects
*  For each one of them
   *  Create OSConfig Guest Policies that will execute a basic command (as an illustrative example)

# How to use

From the [Cloud Shell](https://cloud.google.com/shell)

## Clone the repository

Clone the Git repository with the command

```
git clone ssh://username@gmail.com@source.developers.google.com:2022/p/scip-deployment-manager-dev/r/terraform-multi-project-osconfig-guest-policy
```

change directory, into the repository

```
cd terraform-multi-project-osconfig-guest-policy
```

## Configure Authorization

A service account ought to be authorized to perform operations in Google Cloud
infrastructure.

### Create Custom IAM Roles

In order to assign all the necessary permissions to the service account,
[create an IAM custom 
role](https://cloud.google.com/sdk/gcloud/reference/beta/iam/roles/create)
using the following commands in the script:

```
preparation_scripts/create_terraform_custom_role.sh
enable_services_in_admin_project.sh
```

Where the `TerraformDeployer.yaml` file in this repository already specifies all the permissions needed.

### Create Service Account and assign Custom IAM Role

Use the commands in the script:

```
preparation_scripts/create_terraform_service_account.sh
```

in order to:

*  Create a dedicated service account
*  Assign to it the Custom IAM Role
*  Download the service account key

which follows the GCP documentation for

*  [Creating service accounts](https://cloud.google.com/sdk/gcloud/reference/iam/service-accounts/create).
*  [Binding IAM policies](https://cloud.google.com/sdk/gcloud/reference/projects/add-iam-policy-binding).
*  [Creating service account keys](https://cloud.google.com/sdk/gcloud/reference/iam/service-accounts/keys/create).


### Enable required services

Use the command in the script

```
preparation_scripts/enable_services_in_admin_project.sh
```

to enable the API services required for this tutorial.


### Set up environment variables

As a helper example, use the file

```
preparation_scripts/setup_env.sh
```

Edit the file to introduce the appropriate values in the environment variables.

Then use the command

```
source preparation_scripts/setup_env.sh
```

### Create Resources in order

You can now proceed to create the cloud resouces by using the following modules in order:

```
create_projects
enable_projects_for_vmmanager
create_guest_policies
create_patch_deployments
create_vm_instances
```

Note that the last one `create_vm_instance` could be done either before or after `create_guest_policies` and `create_path_deployments`.


