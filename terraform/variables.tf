variable "region" {
  type        = string
  description = "AWS region"
  default     = "us-east-1"
}

variable "tags" {
  type        = map(string)
  description = "Tags to apply to all resources"
  default     = {}
}

variable "db_password" {
  description = "Password for the main RDS Postgres instance"
  type        = string
  sensitive   = true
}

variable "db_username" {
  description = "Username for the main RDS Postgres instance"
  type        = string
  sensitive   = true
}
