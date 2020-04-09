{{/* Template Info
---
name: describe_mount_response
description: respond to describe mount request
---
*/}}
{
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "plain_text",
        "text": "You selected {{ CachedByKey .InteractionData.option }}",
        "emoji": true
      }
    },
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "*<{{ .ViewUrl }}/mounts/?mnt={{ CachedByKey .InteractionData.option }} | Mount Overview>* | *<{{ .ViewUrl }}/mountdetail/?mnt={{ CachedByKey .InteractionData.option }} | Mount Details>* | *<{{ .ViewUrl }}| Mount Anomalies>*"
        }
    }
  ]
}