{{/* Template Info
---
name: describe_mount
description: describe a mount
---
*/}}
{
  "blocks": [
    {
      "elements": [
        {
          "emoji": true,
          "text": "atsu.io",
          "type": "plain_text"
        }
      ],
      "type": "context"
    },
    {
      "type": "divider"
    },
    {
      "accessory": {
        "action_id": "010|_describe_mount_response",
        "options": [
{{ range $idx,$val := GetMounts .ElasticSearchUrl "*summary*" }}
        {{ if ne $idx 0 }},{{ end }}
          {
            "text": {
              "emoji": true,
              "text": "{{ TruncPath $val 50 }}",
              "type": "plain_text"
            },
            "value": "{{ CachedByVal $val }}"
          }
{{ end }}
        ],
        "placeholder": {
          "emoji": true,
          "text": "Select a mount",
          "type": "plain_text"
        },
        "type": "static_select"
      },
      "text": {
        "text": "Mount:",
        "type": "mrkdwn"
      },
      "type": "section"
    },
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "*<{{ .ViewUrl }}/mounts/|Mount Overview>* | *<{{ .ViewUrl }}/mountdetail/|Mount Details>* | *<{{ .ViewUrl }}|Mount Anomalies>*"
        }
    }
  ]
}