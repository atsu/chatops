{{/* Template Info
This template is for alerting a slack channel with a mount alert.
It should only be triggered by an 'AtsuEvent'
Ex: curl -X POST '<chatopshost>/slack/atsu-event?tpl=_mount_alert.tpl' \
-d '{
    "atsu_id":"<atsu id>",
    "view_path":"<view path>",
    "header": "header text here",
    "tables":[
        ["a","b"],
        ["c","d"]
    ],
    "image_alt": "test",
    "image_url":"https://api.slack.com/img/blocks/bkb_template_images/goldengate.png"
}'
atsu_id, header, and tables are required fields. image_alt is optional but may not be without image_url.
---
name: mount_alert
description: display a mount alert
sendtokafka: true
---
*/}}
{{ if not .InteractionData.atsu_id }}{{ Error "atsu_id is required" }}{{ end }}
{{ if not .InteractionData.header }}{{ Error "header is required" }}{{ end }}
{{ if not .InteractionData.tables }}{{ Error "tables is required" }}{{ end }}
{
  "blocks": [
    {
          "type": "section",
          "text": {
              "type": "mrkdwn",
              "text": "{{ .InteractionData.header }}"
           }
    },
    {{ range $tidx,$table := .InteractionData.tables }}
    {
        "type": "section",
        "fields": [
        {{ range $fidx,$field := $table }}
            {
                "type": "mrkdwn",
                "text": "{{ $field }}"
            }{{ if gt (len $field) 0 }},{{ end }}
        {{ end }}
        ]
    },
    {
        "type": "divider"
    },
    {{ end }}
    {{ if .InteractionData.image_url }}
    {
        "type": "image",
        "image_url": "{{ .InteractionData.image_url }}",
        "alt_text": {{ if .InteractionData.image_alt }} "{{ .InteractionData.image_alt }}" {{ else }}" "{{end}}
    },
    {{ end }}
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
                    "value": "{\"label\":1,\"atsu_id\":\"{{.InteractionData.atsu_id}}\",\"etype\":\"mount_alert\"}",
                    "action_id": "000|_alert_response"
                },
                {
                    "type":"button",
                    "text": {
                        "type": "plain_text",
                        "text": ":thumbsdown:"
                    },
                    "value": "{\"label\":0,\"atsu_id\":\"{{.InteractionData.atsu_id}}\",\"etype\":\"mount_alert\"}",
                    "action_id": "001|_alert_response"
                }
            ]
        }
  ]
}