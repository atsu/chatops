{{/* Template Info
---
name: testing
description: testing
---
*/}}
{
    "options": [
{{ range $idx,$val := GetMounts .ElasticSearchUrl "*summary*" }}
        {{ if ne $idx 0 }},{{ end }}
        {
          "label": "{{ TruncPath $val 50 }}",
          "value": "{{ TruncPath $val 50 }}"
        }
{{ end }}
	]
}