# Go import redirector

## Summary
Generates redirection for go-import by provided prefixes (-p and -r).

Program will only accept requests with `?go-get=1` query.

Example: If `go import bla-bla.bla/package` executed and `-r https://some.root/cgit` set,
request will return `https://some.root/cgit/bla-bla.bla/package`.

If base path for imported module is not provided,
domain name used as base path to generate redirection.

Example: If `go import some.root/package` executed and `-r https://some.root/cgit` set,
request will return `https://some.root/cgit/some.root/package`.

It also generates go-source meta information with dir (tree) and file (blob) links like github.

Useful when VCS deployed in sub path inside root domain name (some.domain/vcs).