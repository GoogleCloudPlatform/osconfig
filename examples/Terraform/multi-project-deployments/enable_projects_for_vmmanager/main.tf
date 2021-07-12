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

data "google_project" "listed_in_folder" {
  count = length(data.google_projects.in_folder.projects)

  project_id = data.google_projects.in_folder.projects[count.index].project_id
}

locals {
  projects  = compact(data.google_project.listed_in_folder.*.number)
}

module "project-services" {
  source  = "terraform-google-modules/project-factory/google//modules/project_services"

  count = length(data.google_projects.in_folder.projects)

  project_id = data.google_projects.in_folder.projects[count.index].project_id

  enable_apis = true
  activate_apis = [
    "iam.googleapis.com",
    "logging.googleapis.com",
    "osconfig.googleapis.com",
    "containeranalysis.googleapis.com",
  ]
}


resource "google_project_service" "compute_api" {
  count = length(data.google_projects.in_folder.projects)

  project = data.google_projects.in_folder.projects[count.index].project_id

  service = "compute.googleapis.com"
  # Wait for some time after the API has been enabled before continuing, as the
  # call returns before the API has actually finished initializing.
  provisioner "local-exec" {
    command ="sleep 60"
  }
}


resource "google_compute_project_metadata_item" "osconfig_enable_meta" {
  count = length(data.google_projects.in_folder.projects)

  project = data.google_projects.in_folder.projects[count.index].project_id

  key     = "enable-osconfig"
  value   = "TRUE"
  depends_on = [ google_project_service.compute_api ]
}

resource "google_compute_project_metadata_item" "osconfig_log_level_meta" {
  count = length(data.google_projects.in_folder.projects)

  project = data.google_projects.in_folder.projects[count.index].project_id

  key     = "osconfig-log-level"
  value   = "debug"
  depends_on = [ google_project_service.compute_api ]
}

resource "google_compute_project_metadata_item" "enable_guest_attributes_meta" {
  count = length(data.google_projects.in_folder.projects)

  project = data.google_projects.in_folder.projects[count.index].project_id

  key     = "enable-guest-attributes"
  value   = "TRUE"
  depends_on = [ google_project_service.compute_api ]
}

