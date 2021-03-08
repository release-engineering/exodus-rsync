exodus-rsync
============

exodus-aware drop-in replacement for rsync.

[![Coverage Status](https://coveralls.io/repos/github/release-engineering/exodus-rsync/badge.svg?branch=main)](https://coveralls.io/github/release-engineering/exodus-rsync?branch=main)


Overview
--------

exodus-rsync is a command-line file transfer tool which is partially compatible with
[rsync](https://rsync.samba.org/).

Rather than transferring content via the rsync protocol, exodus-rsync uploads and
publishes content via [exodus-gw](https://github.com/release-engineering/exodus-gw).

See [exodus architecture](https://release-engineering.github.io/exodus-lambda/arch.html)
for more information on how exodus-rsync works together with other projects in the
Exodus CDN family of projects.


Installation
------------

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


Configuration
-------------

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

(TODO) If the configuration file is absent, exodus-rsync will pass through all commands
to rsync without any usage of exodus-gw.


Usage
-----

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

- exodus-rsync supports only the following rsync arguments, most of which do not have any
  effect.

  | Argument | Notes |
  | -------- | ----- |
  | -v, --verbose | increase log verbosity |
  | -n, --dry-run | dry-run mode, don't upload or publish anything |
  | -r, --recursive | ignored; exodus-rsync is always recursive |
  | -t, --times | ignored |
  | --delete | ignored; deleting content is not supported |
  | -K, --keep-dirlinks | ignored; there are no directories on exodus CDN |
  | -O, --omit-dir-times | ignored; there are no directories on exodus CDN |
  | -z, --compress | ignored |
  | -i, --itemize-changes | (TODO) ignored |
  | -e, --rsh=STRING | ignored; ssh is not used |
  | -L, --copy-links | ignored; exodus-rsync always follows links |
  | --stats | ignored |
  | --timeout=INT | ignored |
  | -a, --archive | ignored |


License
-------

This program is free software: you can redistribute it and/or modify it under the terms
of the GNU General Public License as published by the Free Software Foundation,
either version 3 of the License, or (at your option) any later version.
