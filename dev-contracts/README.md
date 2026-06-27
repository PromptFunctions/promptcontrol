# dev-contracts

`dev-contracts/` contains the three parts that make up Prompt Control.

## Packages

- [`contracts/`](./contracts/README.md)
  Parses SCML and produces render, schema, validation, and policy views.
- [`forge/`](./forge/README.md)
  The default engine most applications should use.
- [`gating/`](./gating/README.md)
  The low-level validator used by `forge`.

## Where To Start

If you are integrating Prompt Control into an application, start with [`forge`](./forge/README.md).

If you are working on SCML itself, go to [`contracts`](./contracts/README.md).
