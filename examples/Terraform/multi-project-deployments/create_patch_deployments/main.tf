data "google_active_folder" "terraform_osconfig" {
  display_name = var.folder_name
  parent       = "organizations/${var.organization_id}"
}

locals {
  folder_id = split("/",data.google_active_folder.terraform_osconfig.id)[1]
}

data "google_projects" "in_folder" {
  filter = "parent.id:${local.folder_id}"
}

data "google_project" "list_in_folder" {
  count = length(data.google_projects.in_folder.projects)

  project_id = data.google_projects.in_folder.projects[count.index].project_id
}

locals {
  projects = compact(data.google_project.list_in_folder.*.number)
}

resource "google_os_config_patch_deployment" "patch_deployments" {
  patch_deployment_id = "patch-deploy-inst"

  count = length(data.google_projects.in_folder.projects)

  project = data.google_projects.in_folder.projects[count.index].project_id

  instance_filter {
    group_labels {
      labels = var.labels
      }
    }

  one_time_schedule {
    # Execute 2 minutes from now
    execute_time = timeadd(timestamp(), var.patch_deployment_execute_time)
  }
}
