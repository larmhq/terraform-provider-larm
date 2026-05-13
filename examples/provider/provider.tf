terraform {
  required_providers {
    larm = {
      source  = "larmhq/larm"
      version = "~> 0.1"
    }
  }
}

provider "larm" {
  # api_key is also read from the LARM_API_KEY environment variable
  api_key = var.larm_api_key
}

variable "larm_api_key" {
  type      = string
  sensitive = true
}
