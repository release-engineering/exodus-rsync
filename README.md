# exodus-rsync

exodus-aware drop-in replacement for rsync.

[![Coverage Status](https://coveralls.io/repos/github/release-engineering/exodus-rsync/badge.svg?branch=main)](https://coveralls.io/github/release-engineering/exodus-rsync?branch=main)

<!-- TOC -->

- [Overview](#overview)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Differences from rsync](#differences-from-rsync)
  - [Publish modes](#publish-modes)
    - [Standalone publish](#standalone-publish)
    - [Joined publish](#joined-publish)
- [License](#license)

<!-- /TOC -->

## Overview

exodus-rsync is a command-line file transfer tool which is partially compatible with
[rsync](https://rsync.samba.org/).

Rather than transferring content via the rsync protocol, exodus-rsync uploads and
publishes content via [exodus-gw](https://github.com/release-engineering/exodus-gw).

See [exodus architecture](https://release-engineering.github.io/exodus-lambda/arch.html)
for more information on how exodus-rsync works together with other projects in the
Exodus CDN family of projects.


## Installation

exodus-rsync is a standalone linux-amd64 binary which may be downloaded from the
[repository releases](https://github.com/release-engineering/exodus-rsync/releases).

It is designed to be installed as the `rsync` command in `$PATH`, ahead of the real
`rsync` command. In typical scenarios this can be accomplished by installing to
`/usr/local/bin`, as in example:

```
curl -LO https://github.com/release-engineering/exodus-rsync/releases/latest/download/exodus-rsync
chmod +x exodus-rsync
mv exodus-rsync /usr/local/bin/rsync
```

In order for exodus-rsync to do anything useful, it's necessary to first deploy a
configuration file; see the next section.


## Configuration

exodus-rsync uses a configuration file found at either:

- exodus-rsync.conf
- $HOME/.config/exodus-rsync.conf
- /etc/exodus-rsync.conf
- path given by `--exodus-conf` command-line argument

The configuration file is written in YAML. The available config keys
are documented in the example below:

```yaml
###############################################################################
# exodus-gw environment settings
###############################################################################
#
# X509 PEM-format certificate and key for authentication to exodus-gw.
# Environment variables may be used with these paths.
gwcert: $HOME/certs/$USER.crt
gwkey: $HOME/certs/$USER.key

# Base URL of the exodus-gw service to be used.
gwurl: https://exodus-gw.example.com

# Defines the exodus-gw "environment" for use.
#
# This value must match one of the environments configured on that service, see:
# https://release-engineering.github.io/exodus-gw/deployment.html#settings
#
# Additionally, the `gwcert` in use must grant the necessary roles for writing to
# this environment, such as `prod-blob-uploader`, `prod-publisher`.
gwenv: prod

###############################################################################
# Environment configuration
###############################################################################
#
# When exodus-rsync is run as 'rsync' it will inspect the target
# user@host component of the command-line.
#
# If this component matches one of the configured prefixes, usage of
# exodus-gw is enabled and the specified config is used. Otherwise,
# exodus-rsync will delegate commands to the real rsync.
environments:

  # Defining a prefix like this enables explicitly syncing to exodus CDN,
  # as in example:
  #
  #   rsync /my/src/tree exodus:/my/dest
  #
  # "prefix" is the only mandatory key here.
- prefix: exodus

  # Defining a prefix like this enables overriding publishes to existing non-exodus
  # targets and diverting them instead to exodus CDN, as in example:
  #
  #   rsync /my/src/tree upload@example.com:/my/dest
  #
- prefix: upload@example.com

  # All top-level configuration keys can also be overridden per environment;
  # for example, to use a different exodus-gw service & environment:
  gwurl: https://other-exodus-gw.example.com/
  gwenv: stage

###############################################################################
# Rsync configuration
###############################################################################
#
# Defines mode of operation for invoking rsync:
#
# - If "exodus", exodus-rsync only publishes to exodus CDN and does not invoke
#   rsync
#
# - If "rsync", exodus-rsync does not publish to exodus CDN and only invokes rsync
#
# - If "mixed", exodus-rsync both publishes to exodus CDN and also invokes rsync,
#   only exiting successfully if both succeed. Beware of the implications on
#   atomicity (e.g. it is possible for one of these to succeed and the other fail).
#
# - Mode is always forced to "rsync" when no environment is matched.
#
rsyncmode: exodus

###############################################################################
# Logging
###############################################################################
#
# Sets the minimum log level for logs sent to the local system log
# (journald or syslog). One of:
#
# "none"   - no logging
# "debug"  - for debugging exodus-rsync, very verbose
# "trace"  - sets debug level for exodus-rsync and the AWS SDK
# "info"   - outputs messages mostly when writes occur; default, and recommended.
# "warn"   - outputs messages when possible issues are encountered
# "error"  - outputs messages when errors occur
#
# Note that this log level is set independently from the level of verbosity
# sent to stdout/stderr, which is only controlled by the "-v" argument.
#
loglevel: info

#
# Force usage of a specific logger backend.
#
# "journald"       - use journald. Recommended, fully supports structured logging.
# "syslog"         - use syslog. Structured logs are embedded as JSON.
# "auto" or absent - autodetect best logger
#
logger: auto

#
# Diagnostic mode.
#
# In diagnostic mode, exodus-rsync will perform various self-checks and dump
# detailed info on the execution environment at the beginning of each
# invocation.
#
# Diagnostic mode is intended for debugging only. It negatively impacts
# performance and should generally be disabled in production.
#
# The `--exodus-diag` command-line option can also enable diagnostic mode.
#
diag: false

###############################################################################
# Tuning
###############################################################################
#
# The following fields, all optional, may affect the performance of
# exodus-rsync.
#
# They are listed here along with their default values.

# When awaiting an exodus-gw publish task, how long (in milliseconds) should
# we wait between each poll of the task status.
gwpollinterval: 5000

# When adding items onto an exodus-gw publish, what is the maximum number of
# items we'll include in a single HTTP request.
gwbatchsize: 10000
```

In order to publish to exodus CDN it is necessary to configure all of the
`gw*` configuration items, and add at least one entry under `environments`.

If the configuration file is absent, exodus-rsync will pass through all commands
to rsync without any usage of exodus-gw.


## Usage

exodus-rsync provides an interface partially compatible with this form of the rsync
command:

```
exodus-rsync [OPTION]... SRC DEST
```

For example, `exodus-rsync /my/srctree exodus:/my/dest` will publish the content of
the `/my/srctree` directory onto Exodus CDN, using `/my/dest` as the root path for
the content.

In cases where the `DEST` argument does not refer to one of the environments in
exodus-rsync.conf, exodus-rsync will delegate to the real rsync command, passing
through the `SRC`, `DEST` and rsync-compatible `OPTIONs` without modification.


### Differences from rsync

exodus-rsync does not aim to cover all rsync use-cases and has many limitations
compared to rsync, as well as a few unique features not supported by rsync. Here
is a summary of the differences:

- exodus-rsync only supports the "single local SRC, remote DEST" form of the rsync command.
  rsync supports other variants, such as multiple SRC directories or copying from a remote SRC to a local DEST.

- exodus-rsync supports a few additional arguments not supported by rsync. All of these are
  prefixed with `--exodus-` to avoid any clashes.

  | Argument | Notes |
  | -------- | ----- |
  | --exodus-conf=PATH | use this configuration file |
  | --exodus-publish=ID | join content to an existing publish (see "Publish modes") |
  | --exodus-diag | diagnostic mode, outputs various info for troubleshooting |

- exodus-rsync supports only the following rsync arguments, most of which do not have any
  effect.

  | Argument | Notes |
  | -------- | ----- |
  | --verbose, -v | increase log verbosity |
  | --archive, -a | ignored |
  | --recursive, -r | ignored; exodus-rsync is always recursive |
  | --relative, -R | use relative path names |
  | --links, -l | copy symlinks as symlinks without following¹ |
  | --copy-links, -L | follow symlinks |
  | --keep-dirlinks, -K | ignored; there are no directories on exodus CDN |
  | --hard-links, -H | ignored |
  | --perms, -p | ignored |
  | --executability, -E | ignored |
  | --acls, -A | ignored |
  | --xattrs, -X | ignored |
  | --owner, -o | ignored |
  | --group, -g | ignored |
  | --devices | ignored |
  | --specials | ignored |
  | -D | ignored; same as --devices and --specials |
  | --times, -t | ignored |
  | --atimes, -U | ignored |
  | --crtimes, -N | ignored |
  | --omit-dir-times, -O | ignored; there are no directories on exodus CDN |
  | --dry-run, -n | dry-run mode, don't upload or publish anything |
  | --rsh, -e | ignored; ssh is not used |
  | --ignore-existing | ignored; exodus-rsync always skips existing files |
  | --delete | ignored; deleting content is not supported |
  | --prune-empty-dirs, -m | ignored; there are no directories on exodus CDN |
  | --timeout | ignored |
  | --filter  | add a file-filtering RULE (supports "+/-" rules and "/" modifier) |
  | --exclude | exclude files matching this pattern |
  | --include | don't exclude files matching PATTERN | 
  | --files-from | read list of source-file names from FILE |
  | --compress, -z | ignored |
  | --stats | ignored |
  | --itemize-changes, -i | ignored |

1. `--links` has the following restrictions:
   * All links must resolve to an item included within the current publish at the
     time of commit.
     (Note that multiple exodus-rsync commands can participate in a single publish,
     see "Publish modes".)
   * Only a single level of link resolution is permitted. This restriction may be
     revisited in the future.

### Publish modes

exodus-rsync supports two different modes of publishing to exodus CDN.

#### Standalone publish

This is the default mode.

exodus-rsync will create a new "publish" object within exodus-gw, add content to it,
and commit it.

In this mode, each individual execution of exodus-rsync will have atomic semantics,
but a sequence of publishes will not be atomic. For example, if we run commands
in sequence:

```
$ exodus-rsync src1 exodus:/dest1
$ exodus-rsync src2 exodus:/dest2
$ exodus-rsync src3 exodus:/dest3
```

... each of dest1, dest2 and dest3 will either be fully exposed on the CDN
or not exposed at all, but if interrupted part way through, it is possible that
(for example) dest1 and dest2 are published but dest3 is not.

#### Joined publish

This mode is activated by calling exodus-rsync with the `--exodus-publish=<publish_id>`
argument.

The given publish ID must have been created in exodus-gw prior to calling exodus-rsync.
exodus-rsync will add content onto the publish, but will not commit it.

In this mode, it is possible to achieve atomic behavior covering a group of exodus-rsync
commands, as in example:

```
# Create a publish.
$ curl [...] -X POST https://exodus-gw.example.com/prod/publish
{"id":"4e59c1a0", ...}

# Let several syncs join this publish.
$ exodus-rsync --exodus-publish 4e59c1a0 src1 exodus:/dest1
$ exodus-rsync --exodus-publish 4e59c1a0 src2 exodus:/dest2
$ exodus-rsync --exodus-publish 4e59c1a0 src3 exodus:/dest3

# Commit the publish
$ curl [...] -X POST https://exodus-gw.example.com/prod/publish/4e59c1a0/commit
{"id":"fa9c4b26", ...}

# (...and we should wait for task fa9c4b26 to complete as well)
```

In the above example, it is ensured that either *all* of dest1, dest2 and dest3 are fully
exposed from the CDN or that *none* of them are exposed at all, even if we are interrupted
in the middle of publishing.  None of the published content becomes visible from the CDN until
the "commit" operation occurs, which exposes all content at once.

See [the exodus-gw documentation](https://release-engineering.github.io/exodus-gw/api.html#section/Atomicity)
for more information on the atomicity guarantees when publishing with
exodus-rsync and exodus-gw.

## License

This program is free software: you can redistribute it and/or modify it under the terms
of the GNU General Public License as published by the Free Software Foundation,
either version 3 of the License, or (at your option) any later version.
