resource "google_folder" "tf_osconfig_folder" {
  display_name = "tf-osconfig-test-folder"
  parent       = "organizations/${var.organization_id}"
}

locals {
  number_of_projects = 3
}

module "project-tf" {
  source = "terraform-google-modules/project-factory/google"

  count = local.number_of_projects

  name              = "tf-osconfig-test-${count.index}"
  random_project_id = "true"
  org_id            = var.organization_id
  billing_account   = var.billing_account
  folder_id         = google_folder.tf_osconfig_folder.id
}

module "project-services" {
  source  = "terraform-google-modules/project-factory/google//modules/project_services"

  count = local.number_of_projects

  project_id = module.project-tf[count.index].project_id

  enable_apis = true
  activate_apis = [
    "iam.googleapis.com",
    "logging.googleapis.com",
    "osconfig.googleapis.com",
    "containeranalysis.googleapis.com",
  ]
}

resource "google_project_service" "compute_api" {
  count = local.number_of_projects

  project = module.project-tf[count.index].project_id

  service = "compute.googleapis.com"
  # Wait for some time after the API has been enabled before continuing, as the
  # call returns before the API has actually finished initializing.
  provisioner "local-exec" {
    command ="sleep 60"
  }
}

resource "google_compute_project_metadata_item" "osconfig_enable_meta" {
  count = local.number_of_projects

  project = module.project-tf[count.index].project_id

  key     = "enable-osconfig"
  value   = "TRUE"
  depends_on = [ google_project_service.compute_api ]
}

resource "google_compute_project_metadata_item" "osconfig_log_level_meta" {
  count = local.number_of_projects

  project = module.project-tf[count.index].project_id

  key     = "osconfig-log-level"
  value   = "debug"
  depends_on = [ google_project_service.compute_api ]
}

resource "google_compute_project_metadata_item" "enable_guest_attributes_meta" {
  count = local.number_of_projects

  project = module.project-tf[count.index].project_id

  key     = "enable-guest-attributes"
  value   = "TRUE"
  depends_on = [ google_project_service.compute_api ]
}
