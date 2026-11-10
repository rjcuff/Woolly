# Contributing to Woolly

Thanks for helping out. Bug reports, fixes, docs and new ideas are all welcome.

## Before you start

For anything bigger than a small fix, open an issue first so we can agree on the approach. It saves you from writing code that doesn't get merged.

## Setup

You need Go 1.22 or newer. Node 22 is only needed for the website.

```sh
git clone https://github.com/rjcuff/wooly
cd wooly
make build      # builds bin/woolly
make test       # runs go vet and the tests
```

Try your build without touching your real memories:

```sh
WOOLLY_HOME=/tmp/woolly-dev ./bin/woolly remember "test memory"
```

### Website and docs

```sh
cd web
npm install
npm run dev     # http://localhost:5173
```

Docs live in the top-level `docs` folder as MDX. Add a page there and list it in `docs/meta.json`.

The site builds to static files in `web/build/client`. It deploys on Vercel with the project root set to `web`, using the settings in `web/vercel.json`.

## Guidelines

- Keep Woolly a single binary with no outside Go dependencies. If you need one, explain why in the issue.
- Memories must stay plain Markdown files that people can read and edit by hand.
- Nothing may send data off the user's machine.
- Add tests for new behavior. Run `make test` before you open a pull request.
- Run `gofmt` on Go code.
- Write docs in plain American English. Short sentences, no jargon.

## Pull requests

1. Fork the repo and create a branch from `main`.
2. Make your change with tests.
3. Make sure `make test` and `cd web && npm run build` pass.
4. Open a pull request that says what changed and why.

## Reporting bugs

Open an issue with:

- What you ran
- What you expected
- What happened instead
- The output of `woolly doctor`

## Security

Please don't report security problems in public issues. Email the maintainers instead, and we'll respond as quickly as we can.

## License

By contributing, you agree that your work is released under the MIT license.
