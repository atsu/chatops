{{/* Template Info
---
name: describe_job
description: describe a job
---
*/}}
{"blocks": [
	{
		"type": "context",
		"elements": [
			{
				"type": "plain_text",
				"text": "atsu.io",
				"emoji": true
			}
		]
	},
	{
		"type": "divider"
	},
	{
		"type": "section",
		"text": {
			"type": "mrkdwn",
			"text": "Mount:"
		},
		"accessory": {
			"type": "static_select",
			"placeholder": {
				"type": "plain_text",
				"text": "Select a mount",
				"emoji": true
			},
			"options": [
				{
					"text": {
						"type": "plain_text",
						"text": "/usr/anim/peep",
						"emoji": true
					},
					"value": "/usr/anim/peep"
				},
				{
					"text": {
						"type": "plain_text",
						"text": "/usr/anim/foo",
						"emoji": true
					},
					"value": "/usr/anim/foo"
				},
				{
					"text": {
						"type": "plain_text",
						"text": "/usr/anim/bar",
						"emoji": true
					},
					"value": "/usr/anim/bar"
				}
			]
		}
	},
	{
		"type": "actions",
		"elements": [
			{
				"type": "button",
				"text": {
					"type": "plain_text",
					"text": "Mount Overview",
					"emoji": true
				},
				"value": "click_me_123"
			},
            			{
				"type": "button",
				"text": {
					"type": "plain_text",
					"text": "Mount Details",
					"emoji": true
				},
				"value": "click_me_123"
			},
            			{
				"type": "button",
				"text": {
					"type": "plain_text",
					"text": "Mount Anomalies",
					"emoji": true
				},
				"value": "click_me_123"
			}
		]
	}
]}
