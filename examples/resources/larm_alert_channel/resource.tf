resource "larm_alert_channel" "webhook" {
  name = "Ops Webhook"
  type = "webhook"

  webhook = {
    url = "https://ops.example.com/larm-alerts"

    headers = {
      Authorization = "Bearer ${var.webhook_token}"
    }
  }
}

resource "larm_alert_channel" "slack" {
  name = "Slack #alerts"
  type = "slack"

  slack = {
    integration_id = var.slack_integration_id
    channel_id     = "C0123456789"
    channel_name   = "#alerts"
  }
}

resource "larm_alert_channel" "discord" {
  name = "Discord"
  type = "discord"

  discord = {
    webhook_url = var.discord_webhook_url
  }
}

resource "larm_alert_channel" "email" {
  name = "Engineering Team"
  type = "email"

  email = {
    recipients = ["oncall@example.com", "sre@example.com"]
  }
}

resource "larm_alert_channel" "ilert" {
  name = "ilert"
  type = "ilert"

  ilert = {
    api_key = var.ilert_api_key
  }
}

resource "larm_alert_channel" "incident_io" {
  name = "incident.io"
  type = "incident_io"

  incident_io = {
    alert_source_url = "https://api.incident.io/v2/alert_events/http/abc123"
    api_token        = var.incident_io_api_token
  }
}

resource "larm_alert_channel" "grafana_irm" {
  name = "Grafana IRM"
  type = "grafana_irm"

  grafana_irm = {
    integration_url = var.grafana_irm_integration_url
  }
}

resource "larm_alert_channel" "mattermost" {
  name = "Mattermost"
  type = "mattermost"

  mattermost = {
    webhook_url = var.mattermost_webhook_url
  }
}

resource "larm_alert_channel" "pagerduty" {
  name = "PagerDuty"
  type = "pagerduty"

  pagerduty = {
    integration_key = var.pagerduty_integration_key
  }
}

resource "larm_alert_channel" "pushover" {
  name = "Pushover"
  type = "pushover"

  pushover = {
    user_key  = var.pushover_user_key
    api_token = var.pushover_api_token
  }
}

resource "larm_alert_channel" "teams" {
  name = "Microsoft Teams"
  type = "teams"

  teams = {
    webhook_url = var.teams_webhook_url
  }
}

resource "larm_alert_channel" "ntfy" {
  name = "ntfy"
  type = "ntfy"

  ntfy = {
    server_url = "https://ntfy.sh"
    topic      = "my-larm-alerts"
  }
}

resource "larm_alert_channel" "telegram" {
  name = "Telegram"
  type = "telegram"

  telegram = {
    bot_token = var.telegram_bot_token
    chat_id   = "-1001234567890"
  }
}
