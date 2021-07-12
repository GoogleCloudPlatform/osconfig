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


locals {
  scopes = [
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


resource "google_compute_network" "vpc_network" {
  name    = "vpc-network"
  count   = length(data.google_projects.in_folder.projects)
  project = data.google_projects.in_folder.projects[count.index].project_id
}

resource "google_compute_firewall" "default" {
  name    = "ssh-firewall-rule"
  count   = length(data.google_projects.in_folder.projects)
  project = data.google_projects.in_folder.projects[count.index].project_id
  network = google_compute_network.vpc_network[count.index].name
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

resource "google_compute_address" "external_ip" {
  name    = "external-ip"
  region  = "us-central1"
  count   = length(data.google_projects.in_folder.projects)
  project = data.google_projects.in_folder.projects[count.index].project_id
}

resource "google_service_account" "default" {
  account_id   = "tf-osconfig-vm"
  display_name = "TF OSConfig VM Service Account"
  count        = length(data.google_projects.in_folder.projects)
  project      = data.google_projects.in_folder.projects[count.index].project_id
}

#
#  The following roles are needed for the service account to be able to write instance metadata.
#
resource "google_project_iam_binding" "log_writer" {
  count   = length(data.google_projects.in_folder.projects)
  project = data.google_projects.in_folder.projects[count.index].project_id
  role    = "roles/logging.logWriter"
  members = [
    "serviceAccount:${google_service_account.default[count.index].email}"
  ]
}

resource "google_project_iam_binding" "compute_viewer" {
  count   = length(data.google_projects.in_folder.projects)
  project = data.google_projects.in_folder.projects[count.index].project_id
  role    = "roles/compute.viewer"
  members = [
    "serviceAccount:${google_service_account.default[count.index].email}"
  ]
}

resource "google_project_iam_binding" "compute_instance_admin_v1" {
  count   = length(data.google_projects.in_folder.projects)
  project = data.google_projects.in_folder.projects[count.index].project_id
  role    = "roles/compute.instanceAdmin.v1"
  members = [
    "serviceAccount:${google_service_account.default[count.index].email}"
  ]
}

resource "google_project_iam_binding" "iam_service_account_user" {
  count   = length(data.google_projects.in_folder.projects)
  project = data.google_projects.in_folder.projects[count.index].project_id
  role    = "roles/iam.serviceAccountUser"
  members = [
    "serviceAccount:${google_service_account.default[count.index].email}"
  ]
}

resource "google_compute_instance" "default" {
  name = "tf-osconfig-vm"

  count = length(data.google_projects.in_folder.projects)

  project = data.google_projects.in_folder.projects[count.index].project_id

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
    scopes = local.scopes
  }
  labels = var.labels
}
