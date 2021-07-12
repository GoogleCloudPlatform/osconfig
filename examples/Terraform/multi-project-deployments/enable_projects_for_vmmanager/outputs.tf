output "google_projects" {
  description = "List of projects inside a given folder"
  value       = local.projects
}

output "folder_name" {
  description = "Folder Name"
  value       = data.google_active_folder.terraform_osconfig.display_name
}

output "folder_id" {
  description = "Folder ID"
  value       = local.folder_id
}
