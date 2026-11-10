# Security

## Reporting a problem

Please don't report security problems in public issues.

Report them privately through [GitHub security advisories](https://github.com/rjcuff/woolly/security/advisories/new). Include what you found, how to reproduce it, and which version of Woolly you used (`woolly version`).

You'll get a reply within a few days. Once a fix is released, we'll credit you in the release notes unless you'd rather stay anonymous.

## Supported versions

Security fixes go into the latest release. Update with:

```sh
curl -fsSL https://usewoolly.com/install.sh | sh
```

## How Woolly protects your data

- Memories stay in `~/.woolly` on your computer. Woolly has no servers and no tracking.
- The `~/.woolly` folder and every memory file can only be read by your user account.
- Passwords, API keys, private keys, card numbers and Social Security numbers are refused.
- Text from an AI or an import that reads like instructions to an AI is refused, and recalled memories are labeled as notes, not instructions.
- Forgotten memories go to a trash folder and can be restored.
- The local API only listens on `127.0.0.1`, requires a secret token, and rejects requests from websites.
- The install script verifies downloads against published checksums.

More detail: [usewoolly.com/docs/security](https://usewoolly.com/docs/security).
