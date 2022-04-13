# An image which may be used for building exodus-rsync.
#
# This Containerfile is intentionally left blank (other than the
# FROM statement). The point is only to pin a certain version of
# a base image, in a manner which may be updated by renovatebot.
FROM registry.access.redhat.com/ubi8/go-toolset@sha256:8bdcfa606506e1343fcda2777b7171da9d59c9537396a2454b7e9dc1e855be3e
