# Threat Model

knobs turns untrusted text into an application's configuration. This document lists what it trusts, what it
defends, and how.

## Assets

- **Secret values:** credentials and keys set in the secrets file, env or cli.
- **Configuration integrity:** the resolved configuration an application acts on.

## Trust boundaries

| Source | Trusted to | Controls |
| --- | --- | --- |
| Schema | define every key, type, bound and marking | owned and reviewed by the application; committed and diffed in review |
| Global, user and secrets files | set values on the host | validated against the schema; the secrets file loads only when its owner alone can read it |
| env, cli | set values for one process | converted to the schema type, then validated; a secret on the command line fails |
| Connection values | override allowlisted keys for one connection | refused on an unauthorized connection; clamped to the schema's bounds; never secret keys; every attempt recorded |

## Threats and mitigations

| Threat | Mitigation |
| --- | --- |
| A secret appears in logs, dumps or errors | secrets print as `<redacted>` in every output the engine owns; each language's secret type redacts itself; errors name the key, never the value |
| A secret is committed in a plain config file | a key marked secret set in the global or user file fails loading |
| A typo or unknown key silently changes nothing | unknown keys fail loading, naming the source and key |
| A caller raises a limit or disables a control | connection overrides only on allowlisted keys, on authorized connections, within the schema's bounds |
| Malformed input crashes the parser | every parser of untrusted input has native fuzz targets, run in CI |
| A dependency is compromised | dependencies are pinned and scanned per language; releases carry SBOMs, signatures and provenance |
