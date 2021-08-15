variable "organization_id" {
  description = "Cloud Organization where to create Projects."
  type        = string
}

variable "billing_account" {
  description = "Billing Account to which charge the Projects."
  type        = string
}

variable "labels" {
  type        = map(string)
  description = "Labels, provided as a map"
}
