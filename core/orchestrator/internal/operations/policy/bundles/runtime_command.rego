# Runtime command policy — enforced before every shell exec inside a
# workspace. The runtime PEP submits action == "runtime.exec" with
# context fields:
#   context.cmd            (string, normalized argv[0])
#   context.argv           (array of strings)
#   context.cwd            (string)
#   context.network        (bool — true if egress is requested)
#   context.command_class  (one of read_inspect | build_test_lint |
#                           package_install — set by the runtime
#                           classifier; unknown => unclassified)
package ironflyer

allow_votes["runtime_command"] {
    input.action == "runtime.exec"
    _allowed_command_class
    not _has_denied_binary
    not _requests_privileged
    not _requests_egress_without_allowlist
}

deny[reason] {
    input.action == "runtime.exec"
    not _allowed_command_class
    reason := "runtime_unknown_command_class"
}

deny[reason] {
    input.action == "runtime.exec"
    _has_denied_binary
    reason := "runtime_denied_binary"
}

deny[reason] {
    input.action == "runtime.exec"
    _requests_privileged
    reason := "runtime_privileged_escape_attempt"
}

deny[reason] {
    input.action == "runtime.exec"
    _requests_egress_without_allowlist
    reason := "runtime_egress_not_allowlisted"
}

_allowed_command_class {
    input.context.command_class == "read_inspect"
}
_allowed_command_class {
    input.context.command_class == "build_test_lint"
}
_allowed_command_class {
    input.context.command_class == "package_install"
}

# Binaries that are categorically denied regardless of class. The
# runtime classifier may already filter most of these; the PDP is the
# belt-and-braces last line.
_denied_binaries := {
    "sudo", "su", "doas",
    "mount", "umount", "modprobe", "insmod", "rmmod",
    "docker", "dockerd", "containerd", "ctr",
    "kubectl", "kubeadm",
    "ssh", "scp", "sftp", "telnet",
    "iptables", "ip6tables", "nft",
    "useradd", "usermod", "userdel", "passwd",
    "chroot", "nsenter", "unshare",
}

_has_denied_binary {
    bin := input.context.cmd
    _denied_binaries[bin]
}

# Privileged hints: argv contains escape patterns or device paths.
_requests_privileged {
    arg := input.context.argv[_]
    startswith(arg, "/dev/")
}

_requests_privileged {
    arg := input.context.argv[_]
    arg == "--privileged"
}

# Egress allowlist lives in context.network_allowlist (set by abuse
# scoring / blueprint). If egress is requested but no allowlist is
# present, deny.
_requests_egress_without_allowlist {
    input.context.network == true
    not input.context.network_allowlist
}

risk_for_allow["medium"] {
    input.action == "runtime.exec"
    input.context.command_class == "package_install"
}

risk_for_allow["high"] {
    input.action == "runtime.exec"
    input.context.network == true
}

obligations_for_allow[o] {
    input.action == "runtime.exec"
    input.context.command_class == "package_install"
    o := {"kind": "audit.high_risk_allow", "params": {"class": "package_install"}}
}
