{{/* Template Info
---
name: issue report
description: submit an issue
---
*/}}
{
    "blocks": [
        {
    		"type": "context",
    		"elements": [
    			{
    				"type": "mrkdwn",
    				"text": "You are about to send the following issue report:"
    			}
    		]
    	},
        {
            "type": "section",
            "text": {
                "type": "plain_text",
                "text": "{{ TrimPrefix .InputText "issue report" }}",
                "emoji": true
            }
        },
        {
            "type": "actions",
            "elements": [
                {
                    "type": "button",
                    "action_id": "000|_issue_report_cancel",
                    "text": {
                        "type": "plain_text",
                        "text": "Cancel",
                        "emoji": true
                    },
                    "value": "cancelled"
                },
                {
                    "type": "button",
                    "action_id": "001|_issue_report_submit",
                    "text": {
                        "type": "plain_text",
                        "text": "Submit",
                        "emoji": true
                    },
                    "value": "{{ .InputText }}"
                }
            ]
        }
    ]
}