{{/* Template Info
---
name: issue
description: submit an issue via dialog interaction
dialog: true
---
*/}}
{
  "callback_id": "000|_kafka",
  "title": "Report an Issue",
  "submit_label": "Submit",
  "notify_on_cancel": false,
  "elements": [
    {
      "label": "Subject",
      "name": "subject",
      "type": "text"
    },
    {
      "label": "Description",
      "name": "description",
      "type": "textarea",
      "hint": "don't forget to include distinguishing information such as JID or Mount"
    }
  ]
}