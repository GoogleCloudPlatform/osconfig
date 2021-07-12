output "folder" {
  description = "The ID of the new folder"
  value       = google_folder.tf_osconfig_folder.id
}

output "projects_self_links" {
  description = "List of self-links to created projects"
  value       = ["${module.project-tf.*}"]
}

