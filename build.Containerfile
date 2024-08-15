# An image which may be used for building exodus-rsync.
#
# This Containerfile is intentionally left blank (other than the
# FROM statement). The point is only to pin a certain version of
# a base image, in a manner which may be updated by renovatebot.
#
# TODO: use ubi9/go-toolset once that contains golang >= 1.21.
FROM golang@sha256:4746d26432a9117a5f58e95cb9f954ddf0de128e9d5816886514199316e4a2fb
