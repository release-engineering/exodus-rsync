# Changelog

## Unreleased

- Generate link_to for symlinks if --links

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
