# The provider reads RESEND_API_KEY from the environment if api_key is not set
# in this block. The config value, when present, takes precedence.
provider "resend" {
  # api_key = var.resend_api_key
}
