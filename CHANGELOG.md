# Changelog

All notable changes to Pebbles will be documented in this file.

The format is based on Keep a Changelog, and this project follows SemVer.

## [Unreleased]

### Added


### Changed


### Fixed

## [0.4.0] - 2026-01-21

### Added
- `pb list`, `pb show`, and `pb ready` now support `--json` output.
- `pb reopen` command to reopen closed issues.
- `pb list --blocked` to surface issues blocked by open dependencies.
- `pb list --stale`/`--stale-days` to show inactive open issues with last activity dates.
- `pb self-update` command to install the latest release.
- Markdown rendering for description/body fields in `pb log` output.

### Changed
- Refreshed `pb log` color palette for better contrast.
- Expanded CLI help text across commands.

### Fixed
- Adjusted `pb log` description formatting for consistent output.

## [0.3.0] - 2026-01-20

### Fixed
- Rebuild cache ordering now applies rename events before dependency/status replay.

## [0.2.0] - 2026-01-19

### Added
- pb update can now change type, priority, and description.
- Beads import workflow (spec + implementation).
- Colored output for `pb log`.


## [0.1.0] - 2026-01-19

### Added
- Initial public release.
