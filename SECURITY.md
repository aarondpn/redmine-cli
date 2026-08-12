# Security Policy

## Supported versions

`redmine-cli` is maintained by volunteers on a best-effort basis. Security
fixes are made for the latest release only. Users should update to the latest
release before reporting a problem and should not expect older releases to
receive backported fixes.

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | No |

## Reporting a vulnerability

Please report suspected vulnerabilities through
[GitHub private vulnerability reporting](https://github.com/aarondpn/redmine-cli/security/advisories/new).
Do not open a public issue, discussion, or pull request for an undisclosed
vulnerability.

Include, when possible:

- the affected version and installation method;
- a description of the impact and the conditions required to reproduce it;
- minimal reproduction steps or a proof of concept;
- any suggested mitigation or fix; and
- whether the vulnerability is already public or actively exploited.

Reports about credential disclosure, authentication or authorization bypass,
unsafe file handling, release or update integrity, and unintended MCP access
are especially useful. Reports about vulnerabilities in Redmine itself or in
an unrelated dependency should be sent to that upstream project unless the
issue is exploitable through `redmine-cli` in a way specific to this project.

## What to expect

The maintainers aim to:

- acknowledge a report within 7 days;
- provide an initial assessment within 14 days;
- keep the reporter informed when the status materially changes; and
- coordinate disclosure after a fix or mitigation is available.

These are response targets rather than guaranteed service levels. Resolution
time depends on severity, complexity, maintainer availability, and upstream
dependencies. If a report is accepted, the project will normally prepare a
fix privately, publish an updated release and GitHub security advisory, and
credit the reporter unless they prefer to remain anonymous.

Please allow the maintainers a reasonable opportunity to investigate and
release a fix before public disclosure. If the issue is already being
exploited or disclosure must happen by a particular date, state that clearly
in the report.

## Research guidelines

Security research must use systems and accounts you own or are explicitly
authorized to test. Do not access other people's data, degrade services,
perform denial-of-service testing, use social engineering, or retain sensitive
data. Stop testing and report immediately if you encounter personal data,
credentials, or other confidential information.

The project will not pursue legal action against good-faith research that
follows this policy, avoids harm, and complies with applicable law. This
statement cannot authorize testing of third-party systems, including Redmine
instances, GitHub, package registries, or hosting providers.
