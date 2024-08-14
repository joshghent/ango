terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.0"
    }
  }
  required_version = "~> 1.8.5"
}

provider "digitalocean" {
  token = var.digitalocean_token
}
