# An image which may be used for building exodus-rsync.
#
# This Containerfile is intentionally left blank (other than the
# FROM statement). The point is only to pin a certain version of
# a base image, in a manner which may be updated by renovatebot.
#
# TODO: use ubi9/go-toolset once that contains golang >= 1.23.
FROM golang@sha256:45b43371f21ec51276118e6806a22cbb0bca087ddd54c491fdc7149be01035d5
