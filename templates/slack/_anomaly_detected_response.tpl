{{/* Template Info
---
name: _anomaly_detected_response
description: handles responses to _anomaly_detected
sendtokafka: true
---
*/}}

{
  "blocks": [
  	{
  		"type": "section",
  		"text": {
  			"type": "mrkdwn",
  			"text": "Thank you for your feedback about anomaly <{{ .ViewUrl }}/jobanomaly/?atsu_id={{ .InteractionData.value }} | {{ .InteractionData.value }}>",
  		}
  	}
  ]
}