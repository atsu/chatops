{{/* Template Info
---
name: _alert_response
description: handles responses to _alert
sendtokafka: true
kafkamessagetype: feedback
---
*/}}

{
 "replace_original": false,
  "blocks": [
  	{
  		"type": "section",
  		"text": {
  			"type": "mrkdwn",
  			"text": "Thank you for your feedback!",
  		}
  	}
  ]
}