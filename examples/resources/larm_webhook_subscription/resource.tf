resource "larm_webhook_subscription" "ops" {
  url = "https://ops.example.com/larm-events"

  events = [
    "monitor.state_changed",
    "monitor.created",
    "monitor.deleted",
  ]
}

# Pass the signing secret to whatever needs to verify webhook signatures.
output "ops_signing_secret" {
  value     = larm_webhook_subscription.ops.secret
  sensitive = true
}
