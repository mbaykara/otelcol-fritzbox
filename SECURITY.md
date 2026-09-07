# Security Policy

## Reporting a vulnerability

Please do not open public issues for security reports. Email the maintainer
privately (see the GitHub profile of @mbaykara) with a description and, if
possible, a proof of concept. You can expect an acknowledgement within a few
days.

## Scope notes

- The receiver talks to a Fritz!Box on the local network using credentials
  you configure. It never sends router credentials anywhere except the
  configured endpoint.
- Opt-in metrics can export device identities (hostname, IP, MAC) and your
  public IP. See the Privacy section in the README before enabling them.
- TR-064 digest authentication uses MD5 because the router protocol requires
  it; this is not used for password storage.
