terraform {
  required_version = ">= 1.6.0"

  backend "gcs" {
    bucket = "shiron-dev-terraform"
    prefix = "ai-baton"
  }

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = local.project_id
  region  = local.region
}
