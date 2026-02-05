# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the Go Service Template.

## What is an ADR?

An ADR captures an important architectural decision along with its context and consequences.
ADRs are numbered sequentially and are immutable once accepted - if a decision changes,
a new ADR supersedes the old one.

## ADR Index

| ID                                           | Title                  | Status   | Date       |
| -------------------------------------------- | ---------------------- | -------- | ---------- |
| [ADR-0001](./0001-hexagonal-architecture.md) | Hexagonal Architecture | Accepted | 2026-02-04 |

## Creating a New ADR

1. Copy `template.md` to `NNNN-short-title.md` (use next sequential number)
2. Fill in the template sections
3. Submit for review via pull request
4. Update this README with the new entry

## References

- [Michael Nygard's ADR article](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
- [ADR GitHub organization](https://adr.github.io/)
