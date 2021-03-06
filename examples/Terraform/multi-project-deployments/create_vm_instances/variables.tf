variable "organization_id" {
  description = "Cloud Organization where to create Projects."
  type        = string
}

variable "folder_name" {
  description = "Folder from where to list projects."
  type        = string
}

variable "labels" {
  type        = map(string)
  description = "Labels, provided as a map"
}
