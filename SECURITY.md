# Security policy

## Supported versions

Security fixes are made on the latest release only. Upgrade to the newest version before reporting.

## Reporting a vulnerability

Please do not open a public issue for security problems.

Report privately through GitHub security advisories:
https://github.com/neverlless/taskshed/security/advisories/new

Include the affected version, how to reproduce, and the impact you expect. You should get a response within a few days. Once a fix is released, the advisory will be published with credit to the reporter unless you prefer otherwise.

## Deployment notes

- Set `API_TOKEN` on any instance reachable by people you do not fully trust. Without it, anyone who can reach the server can create, change and delete tasks.
- TaskShed does not terminate TLS. Put it behind a reverse proxy that does, so the bearer token is not sent in clear text.
