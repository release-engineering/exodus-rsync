# Changelog

## Unreleased

- Updated base build image for Go version 1.21
- Updated to Go version 1.21

## 1.11.2 - 2024-04-03

- HTTP requests made via S3 client now support `gwmaxattempts`
- Updated dependencies

## 1.11.1 - 2024-01-11

- Increased default value of `gwmaxattempts`
- Updated dependencies

## 1.11.0 - 2023-12-04

- Introduced `gwmaxattempts`, `gwmaxbackoff` configuration for retrying failed
  HTTP requests

## 1.10.0 - 2023-10-04

- Introduced `--exodus-commit` and `gwcommit` for configuring the commit mode
- Updated dependencies

## 1.9.7 - 2023-04-24

- Updated dependencies

## 1.9.6 - 2023-03-13

- Added Publish ID validation
- Added X-Idempotency-Key header to idempotent requests
- Updated dependencies

## 1.9.5 - 2023-01-30

- Logging: add connection open/close messages
- Fix: version tagging bug
- Add string input validation
- Updated base build image for Go version 1.19
- Updated dependencies
- Updated to Go version 1.19

## 1.9.4 - 2022-11-21

- Updated dependencies

## 1.9.3 - 2022-10-10

- Updated dependencies
- Minor improvements to logging of progress at `INFO` level
- Improved upload process to perform more steps concurrently

## 1.9.2 - 2022-09-29

- Updated dependencies
- Minor improvements to logging of concurrent uploads

## 1.9.1 - 2022-08-02

- Fix: exodus-rsync cannot publish with links error
- Uploads are now parallelized; introduced `uploadthreads` config option

## 1.9.0 - 2022-07-28

- Implement file logger
- Fix: logging is not ASCII-safe
- Fix: syslog causes panic when running in container
- Add Content-Type to publish items
- Fix: exodus-rsync invokes itself when rsync is missing from system

## 1.8.5 - 2022-06-09

- Upgraded AWS SDK

## 1.8.4 - 2022-04-27

- Refactor container build to support pinned base image
- Fix: incorrect destination path for single file publishes

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
