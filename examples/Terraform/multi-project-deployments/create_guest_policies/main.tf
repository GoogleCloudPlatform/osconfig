locals {
  guest_policy_linux = file("${path.module}/guest_policy_bash_script.txt")
}

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
  projects = compact(data.google_project.listed_in_folder.*.number)
}

resource "google_os_config_guest_policies" "guest_policies" {
  provider = google-beta

  count = length(data.google_projects.in_folder.projects)

  guest_policy_id = "tf-test-guest-policy"
  description     = "Test OSConfig Guest Policy in Linux VM instances."

  project = data.google_projects.in_folder.projects[count.index].project_id

  assignment {
    group_labels {
      labels = var.labels
    }
    os_types {
      os_short_name = "DEBIAN"
      os_version    = "9*"
    }
    os_types {
      os_short_name = "UBUNTU"
    }
  }

  recipes {
    name          = "tf-test-recipe-linux"
    desired_state = "INSTALLED"
    install_steps {
      script_run {
        interpreter = "SHELL"
        script      = local.guest_policy_linux
      }
    }
  }
}
