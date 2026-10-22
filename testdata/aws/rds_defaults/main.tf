terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.69.1"
    }
  }
}

provider "aws" {
  region                      = "us-east-1"
  access_key                  = "test"
  secret_key                  = "test"
  skip_credentials_validation = true
  skip_requesting_account_id  = true
  skip_metadata_api_check     = true
  skip_region_validation      = true
}

variable "db_password" {
  type      = string
  sensitive = true
  default   = "SYNTH_DB_PASSWORD_DO_NOT_USE"
}

resource "aws_db_instance" "defaults" {
  identifier          = "synth-rds-defaults"
  allocated_storage   = 20
  engine              = "postgres"
  engine_version      = "16.4"
  instance_class      = "db.t3.micro"
  username            = "synth_admin"
  password            = var.db_password
  skip_final_snapshot = true
}
