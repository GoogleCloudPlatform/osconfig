#
#   Create Folder to contain test projects
#
resource "google_folder" "tf_osconfig_folder" {
  display_name = var.folder_name
  parent       = "organizations/${var.organization_id}"
}

locals {

  number_of_projects = 3

  vm_service_account_scopes = [
    #
    #  Required by OS Config
    #
    "https://www.googleapis.com/auth/cloud-platform",
    #
    # Default scopes
    #   https://cloud.google.com/sdk/gcloud/reference/alpha/compute/instances/set-scopes#--scopes
    "https://www.googleapis.com/auth/devstorage.read_only",
    "https://www.googleapis.com/auth/logging.write",
    "https://www.googleapis.com/auth/monitoring.write",
    "https://www.googleapis.com/auth/pubsub",
    "https://www.googleapis.com/auth/service.management.readonly",
    "https://www.googleapis.com/auth/servicecontrol",
    "https://www.googleapis.com/auth/trace.append",
  ]
}

#
#   Create test projects
#
module "project-tf" {
  source = "terraform-google-modules/project-factory/google"

  count = local.number_of_projects

  name              = "tf-osconfig-test-${count.index}"
  random_project_id = "true"
  org_id            = var.organization_id
  billing_account   = var.billing_account
  folder_id         = google_folder.tf_osconfig_folder.id
}

#
#   Enable services in test projects
#
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

#
#   Create VM instances inside test projects.
#
resource "google_compute_network" "vpc_network" {
  name    = "vpc-network"
  count = local.number_of_projects
  project = module.project-tf[count.index].project_id
}

resource "google_compute_firewall" "default" {
  name    = "ssh-firewall-rule"
  count = local.number_of_projects
  project = module.project-tf[count.index].project_id
  network = google_compute_network.vpc_network[count.index].name
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

resource "google_compute_address" "external_ip" {
  name    = "external-ip"
  region  = "us-central1"
  count = local.number_of_projects
  project = module.project-tf[count.index].project_id
}

resource "google_service_account" "default" {
  account_id   = "tf-osconfig-vm"
  display_name = "TF OSConfig VM Service Account"
  count = local.number_of_projects
  project = module.project-tf[count.index].project_id
}

#
#  The following roles are needed for the service account to be able to write instance metadata.
#
resource "google_project_iam_binding" "log_writer" {
  count = local.number_of_projects
  project = module.project-tf[count.index].project_id
  role    = "roles/logging.logWriter"
  members = [
    "serviceAccount:${google_service_account.default[count.index].email}"
  ]
}

resource "google_project_iam_binding" "compute_viewer" {
  count = local.number_of_projects
  project = module.project-tf[count.index].project_id
  role    = "roles/compute.viewer"
  members = [
    "serviceAccount:${google_service_account.default[count.index].email}"
  ]
}

resource "google_project_iam_binding" "compute_instance_admin_v1" {
  count = local.number_of_projects
  project = module.project-tf[count.index].project_id
  role    = "roles/compute.instanceAdmin.v1"
  members = [
    "serviceAccount:${google_service_account.default[count.index].email}"
  ]
}

resource "google_project_iam_binding" "iam_service_account_user" {
  count = local.number_of_projects
  project = module.project-tf[count.index].project_id
  role    = "roles/iam.serviceAccountUser"
  members = [
    "serviceAccount:${google_service_account.default[count.index].email}"
  ]
}

resource "google_compute_instance" "default" {
  name = "tf-osconfig-vm"
  count = local.number_of_projects
  project = module.project-tf[count.index].project_id

  machine_type = "n1-standard-1"
  zone         = "us-central1-a"

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-9"
    }
  }

  network_interface {
    network = google_compute_network.vpc_network[count.index].name
    access_config {
      nat_ip = google_compute_address.external_ip[count.index].address
    }
  }

  service_account {
    email  = google_service_account.default[count.index].email
    scopes = local.vm_service_account_scopes
  }
  labels = var.labels
}
