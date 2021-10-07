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
git clone https://github.com:GoogleCloudPlatform/osconfig.git
```

change directory, into the repository

```
cd osconfig
cd examples/Terraform/multi-project-deployments
```

## Define environment variables

The following variables will contain sensitive information. Therefore, it is
recommended that you set them dynamically, only for your session.

```
export TF_VAR_organization_id=YOUR_ORG_ID
export TF_VAR_billing_account=YOUR_BILLING_ACCOUNT_ID
export TF_ADMIN_PROJECT=THE_PROJECT_FROM_WHERE_TF_COMMANDS_WILL_BE_SENT
export TF_ADMIN_USER=THE_USER_TYPING_TF_COMMANDS
```

You can find the values for `YOUR_ORG_ID` and `YOUR_BILLING_ACCOUNT_ID` using the following commands:

```
gcloud organizations list
gcloud beta billing accounts list
```

## Configure User Admin and Project Admin

```
cd preparation_scripts
./create_tf_admin_project.sh
```

This script will create a Project that will serve to administer TF commands. It
will enable the required services in that project. Finally, it will also grant
the necessary permissions to the `TF_ADMIN_USER`.


### Create Resources in order

Login as the `TF_ADMIN_USER` and authenticate with the following commands.

```
gcloud auth application-default login
gcloud auth application-default set-quota-project $TF_ADMIN_PROJECT
```

You can now proceed to create the cloud resouces by using the following modules in order:

```
create_projects

create_guest_policies
create_patch_deployments
```

