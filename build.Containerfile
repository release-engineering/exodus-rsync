# An image which may be used for building exodus-rsync.
#
# This Containerfile is intentionally left blank (other than the
# FROM statement). The point is only to pin a certain version of
# a base image, in a manner which may be updated by renovatebot.
#
# TODO: use ubi9/go-toolset once that contains golang >= 1.19.
FROM golang@sha256:04f76f956e51797a44847e066bde1341c01e09054d3878ae88c7f77f09897c4d
