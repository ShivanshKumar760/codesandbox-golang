terraform {
  required_providers {
    vercel = {
        source = "vercel/vercel"
        version = ">= 0.11"
    }
  }
}

provider "vercel" {}


variable "github_repo" {
  type = string 
  description = "Github repo that holds the monorepo , like your name/codeplatform"
}

resource "vercel_project" "web" {
  name = "codeplatform-web"
  framework = "vite"

  root_directory = "apps/web"

  git_repository = {
    type = "github"
    repo = var.github_repo
  }
}

output "project_id" {
  value = vercel_project.web.id
}