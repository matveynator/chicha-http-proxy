<!-- This document explains the Linux-only setup flow for operators. -->

# Linux interactive setup

The `--setup` flag is available only on Linux builds. It runs an interactive wizard that asks whether to enable HTTPS, which domain to secure, and which upstream URL should receive forwarded requests.

After answering the prompts, the setup script checks whether the host uses `systemd` or legacy `init.d` and writes the appropriate service definition. It also appends operational commands to `/etc/info` so operators can start, stop, and view logs quickly.
