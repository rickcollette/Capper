#!/bin/sh
# Wrapper called by capper-run.sh via sudo.
# cap_net_admin       — bridge/veth/netns configuration, moving interfaces between namespaces
# cap_net_raw         — raw sockets for future health-check probes
# cap_sys_admin       — unshare(CLONE_NEWNET) to create per-instance network namespaces
#                       (required by all container runtimes; scoped to network ops only)
# cap_net_bind_service — bind DNS daemon to gateway:53 (privileged port) per network
BINARY="${1:?usage: capper-setcap.sh <path-to-capper-bin>}"
exec /usr/sbin/setcap cap_net_admin,cap_net_raw,cap_sys_admin,cap_net_bind_service+eip "$BINARY"
