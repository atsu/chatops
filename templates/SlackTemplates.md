
Slack templates are built around [slack](https://api.slack.com/) json and [Go Templates](https://golang.org/pkg/text/template)

A template **MUST** begin with a comment section containing the 'Front Matter' or template metadata.
These values are sandwiched between `---` and should be valid yaml
Example of the template header is
```
{{/* Template Info
---
name: the template name
description: the template description
sendtokafka: false
isterminating: false
dialog: false
extra:
  key: value
---
/*}}
```
The values in the metadata are described as the following actions
`sendtokafka` - if true we will send the resulting template data to the `<prefix>.chatops.slack` kafka stream

`isterminating` - if true will prevent executing a follow up template and prevent sending a response

`dialog` - if true the template is considered to be a dialog, and the response sent to slack is done via the `open.dialog` method

`extra` - is a key value store that is not currently used, but can be populated to forward template information to slack (assuming sendtokafka is true)


Template names are used as their command reference, for example the "describe_mount.tpl" 
can be accessed via slash command
```
/atsu describe mount
``` 
or mention 

**note** mentions are currently hard coded to call the `_kafka.tpl` template
```
@atsu describe mount
```
to provide parameters to these commands, a dash `-` is used to signify a parameter for 
example, running `/atsu describe -mount /one/two -test` will resolve to running the
`describe_mount.tpl` template with the resulting parameters
```
mount="/one/two"
test=true
```
to access these parameters via template, they are found in the `InteractionData` field

Ex: `the selected mount  {{ .InteractionData.mount }}`

This is a reference to the `TemplateData` struct which is supplied when executing the templates
One caveat here is that the results of user interaction with a block action template will result
in the following values available as `InteractionData` in the root object in the template execution

| Field          | Contents             |
| ---------------|----------------------|
| text           | Text                 |
| value          | Value                |
| i-channel      | InitialChannel       |
| channel        | SelectedChannel      |
| i-option       | InitialOption        |
| option         | SelectedOption       |
| i-user         | InitialUser          |
| user           | SelectedUser         |
| i-date         | InitialDate          |
| date           | SelectedDate         |
| i-conversation | InitialConversation  |
| conversation   | SelectedConversation |

On Demand Templates
---
Using the template format above, OnDemandTemplates can be created by simply posting a template to 
`/slack/on-demand-template`
example body:
```
{{/* Template Info
---
name: text
description: display text
---
*/}}
{
    "blocks": [
        {
            "type": "section",
            "text": {
                "type": "mrkdwn",
                "text": "{{ .InteractionData.text }}"
            }
        }
    ]
}
```
These templates can be run just the same as any other template. To run an on demand template from the
`/slack/atsu-event` handler add a truthy `od` query parameter and the name of the template
is the name that was specified in the metadata.

Chaining
---
chaining templates together can be done using the `ActionID` or `CallbackID` these are essentially the
only value that we can give slack and get it back on the response. Both these fields share the same
format, `<ID>|<TemplateName>` the ID is required because slack requires this field to be unique.
The TemplateName is the name of the template to chain to.

Helpers
---
helpers available during template processing can be found [here](/bot/helpers.go)

