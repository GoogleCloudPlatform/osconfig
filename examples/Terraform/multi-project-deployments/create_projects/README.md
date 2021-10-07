# Create GCP Project in a Folder

This module is used to create a set of GCP projects in a Folder.

# Usage

## Configure the Variables

The variables required by this module are defined in the `variables.tf` file.

You can provide the specific values desired for your case by defining the environment variables

*   `TF_VAR_organization_id`
*   `TF_VAR_folder_name`
*   `TF_VAR_billing_account`

## Launching the Module

Use the standard commands

```
terraform init
```

```
terraform validate
```

```
terraform plan -out=plan.out
```

Inspect the output, and if you are satisfied, run

```
terraform apply plan.out
```

## Destroying the Resources

Once you no longer have use for the projects, you can destroy them with the command

```
terraform destroy
```
