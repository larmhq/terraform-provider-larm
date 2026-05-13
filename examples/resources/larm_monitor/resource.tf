resource "larm_monitor" "homepage" {
  name       = "Homepage"
  check_type = "http"

  config = jsonencode({
    url                   = "https://larm.dev"
    method                = "GET"
    expected_status_codes = [200]
    follow_redirects      = true
  })
}

resource "larm_monitor" "cron" {
  name       = "Nightly batch"
  check_type = "heartbeat"

  config = jsonencode({
    expected_interval  = 86400
    consecutive_misses = 1
  })
}
