{{/* Template Info
---
name: text
description: display text
---
*/}}
{
    "blocks": [
        {
            "type": "section",
            "text": {
                "type": "mrkdwn",
                "text": "{{ .InteractionData.text }}"
            }
        }
    ]
}