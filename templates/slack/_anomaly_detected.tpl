{{/* Template Info
This template is for alerting a slack channel with a notification of an anomaly.
It should only be triggered by an 'AtsuEvent'
Ex: curl -X POST '<chatopshost>/slack/atsu-event?tpl=_anomaly_detected' -d '{ "atsu_id":"<atsu id>"}'
---
name: anomaly_detected
description: display anomaly alert
---
*/}}
{
  "blocks": [
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "<{{ .ViewUrl }}/jobanomaly/?atsu_id={{ .InteractionData.atsu_id }} | Anomaly Reported!>"
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
                "value": "{{ .InteractionData.atsu_id }}",
                "action_id": "000|_anomaly_detected_response"
            },
            {
                "type":"button",
                "text": {
                    "type": "plain_text",
                    "text": ":thumbsdown:"
                },
                "value": "{{ .InteractionData.atsu_id }}",
                "action_id": "001|_anomaly_detected_response"
            }
        ]
    }
  ]
}