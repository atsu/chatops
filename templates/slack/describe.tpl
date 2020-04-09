{{/* Template Info
---
name: describe
description: list describe options
sendtokafka: false
---
*/}}
{
  "type": "mrkdwn",
{{ if .InteractionData.m }}
  "text": "{{ .ViewUrl }}/mountdetail/?mnt={{ .InteractionData.m }}",
{{ else if .InteractionData.mount }}
  "text": "{{ .ViewUrl }}/mountdetail/?mnt={{ .InteractionData.mount }}",
{{ else if .InteractionData.j }}
  "text": "{{ .ViewUrl }}/jobdetail/?jid={{ .InteractionData.j }}",
{{ else if .InteractionData.job }}
  "text": "{{ .ViewUrl }}/jobdetail/?jid={{ .InteractionData.job }}",
{{ else if .InteractionData.f }}
  "text": "{{ .ViewUrl }}/filedetail/?fh={{ .InteractionData.f }}",
{{ else if .InteractionData.file }}
  "text": "{{ .ViewUrl }}/filedetail/?fh={{ .InteractionData.file }}",
{{ else }}
  "text": "You can describe a *mount*, a *job*, or a *file*.\nExample: `/atsu describe -m /mount/one`",
{{ end }}
  "replace_original": true
}
