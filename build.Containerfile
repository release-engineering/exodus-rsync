# An image which may be used for building exodus-rsync.
#
# This Containerfile is intentionally left blank (other than the
# FROM statement). The point is only to pin a certain version of
# a base image, in a manner which may be updated by renovatebot.
#
FROM golang@sha256:d337ecb3075f0ec76d81652b3fa52af47c3eba6c8ba9f93b835752df7ce62946
