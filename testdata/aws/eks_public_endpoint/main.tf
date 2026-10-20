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

resource "aws_eks_cluster" "public_endpoint" {
  name     = "SYNTH_eks_public_endpoint"
  role_arn = "arn:aws:iam:::role/SYNTH_EKS_ROLE"

  vpc_config {
    subnet_ids              = ["subnet-SYNTH_EKS_A", "subnet-SYNTH_EKS_B"]
    endpoint_public_access  = true
    public_access_cidrs     = ["0.0.0.0/0"]
  }
}
