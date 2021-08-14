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
git clone git@github.com:GoogleCloudPlatform/osconfig.git
```

change directory, into the repository

```
cd examples/Terraform/multi-project-deployments
```

## Define environment variables

The following variables will contain sensitive information. Therefore, it is
recommended that you set them dynamically, only for your session.

```
export TF_VAR_organization_id=YOUR_ORG_ID
export TF_VAR_billing_account=YOUR_BILLING_ACCOUNT_ID
export TF_ADMIN_USER_EMAIL=THE_USER_TYPING_TF_COMMANDS
```

You can find the values for `YOUR_ORG_ID` and `YOUR_BILLING_ACCOUNT_ID` using the following commands:

```
gcloud organizations list
gcloud beta billing accounts list
```

## Configure Authorization

As the user running the Terraform commands, you will need a set of permissions.

### Create Custom IAM Roles

In order to assign all the necessary permissions,
[create an IAM custom 
role](https://cloud.google.com/sdk/gcloud/reference/beta/iam/roles/create)
using the following commands in the script:

```
preparation_scripts/create_terraform_custom_role.sh
```

Where the `TerraformDeployer.yaml` file in this repository already specifies all the permissions needed.

### Create and assign Custom IAM Role to TF admin user

Use the commands in the script:

```
preparation_scripts/grant_terraform_custom_role_to_admin_user.sh
```

in order to assign the new Custom Role to the admin user who will be typing Terraform commands.

which follows the GCP documentation for

*  [Binding IAM policies](https://cloud.google.com/sdk/gcloud/reference/projects/add-iam-policy-binding).


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


