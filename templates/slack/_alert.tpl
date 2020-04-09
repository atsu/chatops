{{/* Template Info
This template is for alerting a slack channel with a notification.
It should only be triggered by an 'AtsuEvent'
Ex: curl -X POST '<chatopshost>/slack/atsu-event?tpl=_alert.tpl' -d '{ "view_path":"jobanomaly", "atsu_id":"<atsu id>","value":1.234, "text":"alert text"}'
---
name: alert
description: display alert
sendtokafka: true
---
*/}}
{{ if not .InteractionData.atsu_id }}{{ Error "atsu_id is required" }}{{ end }}
{{ if not .InteractionData.text }}{{ Error "text is required" }}{{ end }}
{{ if not .InteractionData.value }}{{ Error "value is required" }}{{ end }}
{
  "blocks": [
    {
          "type": "section",
          "text": {
              "type": "mrkdwn",
              "text": "{{ .InteractionData.text }}"
           }
    },
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "<{{ .ViewUrl }}/{{if .InteractionData.view_path }}{{ .InteractionData.view_path }}{{ else }}alertdetail{{ end }}?atsu_id={{ .InteractionData.atsu_id }}{{if .InteractionData.alert_type }}&alert_type={{ .InteractionData.alert_type }}{{end}} | {{ if .InteractionData.label }}{{ .InteractionData.label }}{{ else }}View Alert - *{{ .InteractionData.atsu_id }}*{{ end }}>"
         }
    },
    {
        "type":"actions",
        "elements": [
            {
                "type":"button",
                "text": {
                    "type": "plain_text",
                    "text": ":thumbsup:"
                },
                "value": "{\"value\":{{.InteractionData.value}},\"label\":1,\"atsu_id\":\"{{.InteractionData.atsu_id}}\",\"etype\":\"{{.InteractionData.alert_type}}\"}",
                "action_id": "000|_alert_response"
            },
            {
                "type":"button",
                "text": {
                    "type": "plain_text",
                    "text": ":thumbsdown:"
                },
                "value": "{\"value\":{{.InteractionData.value}},\"label\":0,\"atsu_id\":\"{{.InteractionData.atsu_id}}\",\"etype\":\"{{.InteractionData.alert_type}}\"}",
                "action_id": "001|_alert_response"
            }
        ]
    }
  ]
}