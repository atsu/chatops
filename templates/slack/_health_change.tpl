{{/* Template Info
This template is for alerting a slack channel with a notification of a health state.
It should only be triggered by an 'AtsuEvent'
Ex: curl -X POST '<chatopshost>/slack/atsu-event?tpl=_health_change' -d '{ "health":"red"}'

'environment' is an optional field that can be used to signify which environment is being referenced.
*This template is currently being used by the health service -> github.com/atsu/health
---
name: _health_change
description: display health state change alert
---
*/}}
{
    "attachments": [
        {
{{ if eq .InteractionData.health "blue" }}
            "color": "#0000ff",
{{ else if eq .InteractionData.health "green" }}
            "color": "#00ff00",
{{ else if eq .InteractionData.health "yellow" }}
            "color": "#ffff00",
{{ else if eq .InteractionData.health "red" }}
            "color": "#ff0000",
{{ else if eq .InteractionData.health "gray" }}
            "color": "#999999",
{{ end }}
            "blocks": [
                {
                    "type": "section",
                    "text": {
                        "type": "mrkdwn",
{{ if .InteractionData.environment  }}
                        "text": "{{.InteractionData.environment}} Health"
{{ else }}
                        "text": "<{{ .HealthUrl }}| Health>"
{{ end }}

                    }
                }
            ]
        }
    ]
}