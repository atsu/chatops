{{/* Template Info
---
name: health
description: display health message
---
*/}}
{
  "response_type": "in_channel",
  "replace_original": true,
{{ if .InteractionData.healthurl  }}
  "text": "<{{ .InteractionData.healthurl }}| Health>"
{{ else }}
  "text": "<http://health.atsu.io| Health>"
{{ end }}
}
