resource "larm_monitor" "api" {
  name       = "API"
  check_type = "http"
  config = jsonencode({
    url                   = "https://api.example.com/health"
    method                = "GET"
    expected_status_codes = [200]
  })
}

resource "larm_monitor" "db" {
  name       = "Database"
  check_type = "http"
  config = jsonencode({
    url                   = "https://db.example.com/health"
    method                = "GET"
    expected_status_codes = [200]
  })
}

resource "larm_status_page" "acme" {
  name          = "Acme Status"
  slug          = "acme"
  description   = "Live status of Acme services."
  primary_color = "#E02424"

  components = [
    {
      type = "group"
      name = "Core Services"
      components = [
        {
          name = "API"
          monitors = [
            {
              monitor_id  = larm_monitor.api.id
              down_status = "major_outage"
            }
          ]
        },
        {
          name = "Database"
          monitors = [
            {
              monitor_id = larm_monitor.db.id
            }
          ]
        }
      ]
    },
    {
      type        = "component"
      name        = "Email"
      description = "Transactional email delivery."
    },
    {
      type = "group"
      name = "Web"
      components = [
        { name = "Dashboard" }
      ]
    }
  ]
}
