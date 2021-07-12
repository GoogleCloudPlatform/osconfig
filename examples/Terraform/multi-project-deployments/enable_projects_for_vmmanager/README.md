#  Enable Projects to use VMManager

This module is used to enable projects to use the VMManager functionalities.

This includes

*   Enabling required APIs
*   Defining METADATA at project level

# Usage

## Configure the Variables

Define the folder name in the environment variable: `TF_VAR_folder_name`.

For example:

```
export TF_VAR_folder_name="production-department-x-folder"
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

The resources created by this module can be destroyed with the command:

```
terraform destroy
```
