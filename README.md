# Chatops
Health reporting is available by hitting the root path url ex `localhost:8040/`
as well as on the default kafka topic `<prefix>.health.chatops`

Slack kafka messages are sent to topic `<prefix>.chatops.slack` 

[Slack Templates](templates/SlackTemplates.md)


# Relay
the chatops relay is a component that supports the following modes.
* OFF - this is the default mode in which the relay is not active
* Handle - in this mode chatops will interact with slack and continually attempt to connect to the configured relay host and port. The target should be another chatops instance in passthrough mode
* Passthrough - in this mode chatops will not interact with slack but instead forward any requests to another instance of chatops that is in handler mode and has established a relay connection. **Note** this mode requires certificates to enable tls.

# Other
Generation of self-sign a certificate with a private (.key) and public key (PEM-encodings .pem|.crt) in one command:

```
# ECDSA recommendation key ≥ secp384r1
# List ECDSA the supported curves (openssl ecparam -list_curves)
openssl req -x509 -nodes -newkey ec:secp384r1 -keyout server.ecdsa.key -out server.ecdsa.crt -days 3650
# openssl req -x509 -nodes -newkey ec:<(openssl ecparam -name secp384r1) -keyout server.ecdsa.key -out server.ecdsa.crt -days 3650
# -pkeyopt ec_paramgen_curve:… / ec:<(openssl ecparam -name …) / -newkey ec:…
ln -sf server.ecdsa.key server.key
ln -sf server.ecdsa.crt server.crt

# RSA recommendation key ≥ 2048-bit
openssl req -x509 -nodes -newkey rsa:2048 -keyout server.rsa.key -out server.rsa.crt -days 3650
ln -sf server.rsa.key server.key
ln -sf server.rsa.crt server.crt
```