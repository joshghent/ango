variable "digitalocean_token" {
  type        = string
  description = "DigitalOcean API Token"
  sensitive   = true
}

variable "jwt_secret" {
  type = string
}

variable "db_user" {
  type = string
}

variable "db_password" {
  type      = string
  sensitive = true
}
