# VM Instances

This module is used to create multple VM instances for the purpose of testing
the execution of the OSConfig Guest Policy.

# Usage

## Configure the Variables

*  Define the folder name in the environment variable: `TF_VAR_folder_name`.
*  Define the organization ID in the environment variable: `TF_VAR_organization_id`.

For example:

```
export TF_VAR_folder_name="production-department-x-folder"
export TF_VAR_organization_id="0123456789"
```

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

Once you no longer have use for the VM instances, you can destroy them with the command

```
terraform destroy
```
