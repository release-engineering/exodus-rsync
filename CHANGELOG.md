# Changelog

## Unreleased

- n/a

## 1.8.3 - 2022-02-24

- Fix: exodus-gw error responses truncated with null bytes
- Expand environment variables at environment config level

## 1.8.2 - 2022-02-03

- Fix: `--dry-run` not passed through to rsync

## 1.8.1 - 2021-12-09

- Fix: incorrect path calculation when `--files-from` is used together
  with a source tree without a trailing slash.

## 1.8.0 - 2021-12-07

- Fix: incorrect calculation of link_to values
- Fix: include/exclude matches against full path
- Fix: include/exclude pattern matching differs from rsync

## 1.7.0 - 2021-11-24

- Add documentation for `strip` in environment configuration
- Fix: `--files-from` wrongly duplicates source-spec path in `web_uri`

## 1.6.0 - 2021-11-17

- The destination path now has the `prefix` path stripped (overridable by `strip`
  in configuration)
- `--links` now supports copying links without following them

## 1.5.0 - 2021-11-02

- Fix: ensure complex types are included in syslog messages
- Introduced diagnostic mode (`--exodus-diag`) for troubleshooting

## 1.4.0 - 2021-10-01

- Support --links argument

## 1.3.0 - 2021-09-29

- Fix: correct --atimes, --crtimes names 
- Support --server, --sender internal arguments
- Support --include, --filter arguments

## 1.2.0 - 2021-09-20

- Support --prune-empty-dirs argument
- Support --files-from argument
- Support --relative argument
- Support --exclude arguments
- Integrate AWS SDK debug logging
- Show error responses from exodus-gw

## 1.1.0 - 2021-06-08

- Accept "preserve" family of rsync arguments
- Invoke rsync if config file isn't found
- Fix: panic in dry-run mode when missing cert/key.

## 1.0.0 - 2021-03-11

- First version with stable interface.
- Handling of "-v" option changed slightly to improve consistency with rsync.

## 0.2.1 - 2021-03-10

- Early development version
