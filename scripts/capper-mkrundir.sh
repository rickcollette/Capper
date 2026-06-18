#!/bin/sh
# Called via sudo. Creates /run/capper/netns owned by the invoking user.
# SUDO_UID/SUDO_GID are set by sudo to the calling user's identity.
uid="${SUDO_UID:?SUDO_UID not set}"
gid="${SUDO_GID:?SUDO_GID not set}"
mkdir -p /run/capper/netns
chown "$uid:$gid" /run/capper /run/capper/netns
