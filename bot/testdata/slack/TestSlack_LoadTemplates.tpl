{{/* Template Info
---
name: TestSlack_LoadTemplates
description: a template for testing
sendtokafka: true
kafkamessagetype: feedback
dialog: true
isterminating: true
extra:
  key: value
---
*/}}
{"ElasticSearchUrl":"{{.ElasticSearchUrl}}","ViewUrl":"{{.ViewUrl}}","HealthUrl":"{{.HealthUrl}}","Team":"{{.Team}}","Channel":"{{.Channel}}","User":"{{.User}}","InputText":"{{.InputText}}","Timestamp":{{.Timestamp}},"InteractionData":{"key":"{{.InteractionData.key}}"}}