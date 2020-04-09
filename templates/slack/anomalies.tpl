{{/* Template Info
---
name: anomalies
description: display recent anomalies
sendtokafka: false
---
*/}}
{
  "replace_original": true,
  {{ $anom := GetAnomalies }}
  {{ if gt (len $anom) 0 }}
  "text": "Here are some recent anomalies...\n{{ $anom }}"
  {{ else }}
  "text": "Here are some recent anomalies...\n*None*"
  {{ end}}
}
