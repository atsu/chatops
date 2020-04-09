{{/* Template Info
DO NOT MODIFY
This is the freeform template, that is designed to allow passing arbitrary json to slack
It should only be triggered by an 'AtsuEvent'
Ex: curl -X POST '<chatopshost>/slack/atsu-event?tpl=_freeform.tpl' -d '{"blocks":[{"type": "section","text": {"type": "plain_text","text": "This is a plain text section block.","emoji": true}}]}'
---
name: freeform
description: allow arbitrary json to slack
---
*/}}
{}