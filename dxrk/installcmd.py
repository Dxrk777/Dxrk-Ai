# SPDX-License-Identifier: MIT
from __future__ import annotations

import os
import shutil
import subprocess
import tempfile
from typing import Optional, Protocol

from dxrk.models import AgentID, ComponentID
from dxrk.system import PlatformProfile

__all__ = [
    "InstallError",
    "CommandSequence",
    "Resolver",
    "ProfileResolver",
    "new_resolver",
    "validate_agent_install_preflight",
    "git_bash_path",
]

# Package-level vars for testability (mirrors Go's cmdLookPath, osStat, osGetenv, cmdGoVersion)
_cmd_look_path = shutil.which
_os_stat = os.stat
_os_getenv = os.environ.get


class InstallError(Exception):
    pass


CommandSequence = list[list[str]]


class Resolver(Protocol):
    def resolve_agent_install(
        self, profile: PlatformProfile, agent: AgentID
    ) -> CommandSequence: ...

    def resolve_component_install(
        self, profile: PlatformProfile, component: ComponentID
    ) -> CommandSequence: ...

    def resolve_dependency_install(
        self, profile: PlatformProfile, dependency: str
    ) -> CommandSequence: ...


class ProfileResolver:
    def resolve_agent_install(
        self, profile: PlatformProfile, agent: AgentID
    ) -> CommandSequence:
        if agent == AgentID.CLAUDE_CODE:
            return _resolve_claude_code_install(profile)
        if agent == AgentID.OPENCODE:
            return _resolve_opencode_install(profile)
        if agent == AgentID.KILOCODE:
            return _resolve_kilocode_install(profile)
        if agent == AgentID.KIMI:
            return _resolve_kimi_install(profile)
        raise InstallError(f"install command is not supported for agent {agent!r}")

    def resolve_component_install(
        self, profile: PlatformProfile, component: ComponentID
    ) -> CommandSequence:
        if component == ComponentID.DXRK_MEMORY:
            return _resolve_engram_install(profile)
        if component == ComponentID.DXRK_GUARDIAN:
            return _resolve_gga_install(profile)
        raise InstallError(
            f"install command is not supported for component {component!r}"
        )

    def resolve_dependency_install(
        self, profile: PlatformProfile, dependency: str
    ) -> CommandSequence:
        if not dependency:
            raise InstallError("dependency name is required")

        pm = profile.package_manager
        if pm == "brew":
            return [["brew", "install", dependency]]
        if pm == "apt":
            return [["sudo", "apt-get", "install", "-y", dependency]]
        if pm == "pacman":
            return [["sudo", "pacman", "-S", "--noconfirm", dependency]]
        if pm == "dnf":
            return [["sudo", "dnf", "install", "-y", dependency]]
        if pm == "winget":
            return [
                [
                    "winget",
                    "install",
                    "--id",
                    dependency,
                    "-e",
                    "--accept-source-agreements",
                    "--accept-package-agreements",
                ]
            ]
        raise InstallError(
            f"unsupported package manager {pm!r} for os={profile.os!r} distro={profile.linux_distro!r}"
        )


def new_resolver() -> Resolver:
    return ProfileResolver()


# ---------------------------------------------------------------------------
# Agent install resolvers
# ---------------------------------------------------------------------------


def _resolve_claude_code_install(profile: PlatformProfile) -> CommandSequence:
    if profile.os == "linux" and not profile.npm_writable:
        return [["sudo", "npm", "install", "-g", "@anthropic-ai/claude-code"]]
    return [["npm", "install", "-g", "@anthropic-ai/claude-code"]]


def _resolve_kilocode_install(profile: PlatformProfile) -> CommandSequence:
    if profile.os == "linux" and not profile.npm_writable:
        return [["sudo", "npm", "install", "-g", "@kilocode/cli"]]
    return [["npm", "install", "-g", "@kilocode/cli"]]


def _resolve_kimi_install(profile: PlatformProfile) -> CommandSequence:
    if not profile.supported:
        raise InstallError(
            f"Kimi is not supported on this platform ({profile.os}/{profile.linux_distro})"
        )
    return [["uv", "tool", "install", "--python", "3.13", "kimi-cli"]]


def _resolve_opencode_install(profile: PlatformProfile) -> CommandSequence:
    pm = profile.package_manager
    if pm == "brew":
        return [["brew", "install", "anomalyco/tap/opencode"]]
    if pm in ("apt", "pacman", "dnf"):
        if profile.npm_writable:
            return [["npm", "install", "-g", "opencode-ai"]]
        return [["sudo", "npm", "install", "-g", "opencode-ai"]]
    if pm == "winget":
        return [["npm", "install", "-g", "opencode-ai"]]
    raise InstallError(
        f"unsupported platform for opencode: os={profile.os!r} distro={profile.linux_distro!r} pm={pm!r}"
    )


# ---------------------------------------------------------------------------
# Component install resolvers
# ---------------------------------------------------------------------------


def _resolve_engram_install(profile: PlatformProfile) -> CommandSequence:
    if profile.package_manager == "brew":
        return [
            ["brew", "tap", "Dxrk777/homebrew-tap"],
            ["brew", "install", "DXRK_MEMORY"],
        ]
    raise InstallError(
        f"DXRK_MEMORY on {profile.os!r}/{profile.package_manager!r} uses direct binary download "
        "\u2014 use DXRK_MEMORY.DownloadLatestBinary() instead of CommandSequence"
    )


def _resolve_gga_install(profile: PlatformProfile) -> CommandSequence:
    pm = profile.package_manager
    if pm == "brew":
        return [
            ["brew", "tap", "Dxrk777/homebrew-tap"],
            ["brew", "reinstall", "DXRK_GUARDIAN"],
        ]
    if pm in ("apt", "pacman", "dnf"):
        tmp_dir = "/tmp/gentleman-guardian-angel"
        return [
            ["rm", "-rf", tmp_dir],
            [
                "git",
                "clone",
                "https://github.com/Dxrk777/gentleman-guardian-angel.git",
                tmp_dir,
            ],
            ["bash", f"{tmp_dir}/install.sh"],
        ]
    if pm == "winget":
        clone_dst = os.path.join(tempfile.gettempdir(), "gentleman-guardian-angel")
        bash = _git_bash_path()
        return [
            [
                "powershell",
                "-NoProfile",
                "-Command",
                f"Remove-Item -Recurse -Force -ErrorAction SilentlyContinue '{clone_dst}'; exit 0",
            ],
            [
                "git",
                "clone",
                "https://github.com/Dxrk777/gentleman-guardian-angel.git",
                clone_dst,
            ],
            [bash, _bash_script_path(profile, os.path.join(clone_dst, "install.sh"))],
        ]
    raise InstallError(
        f"unsupported platform for gga: os={profile.os!r} distro={profile.linux_distro!r} pm={pm!r}"
    )


# ---------------------------------------------------------------------------
# Preflight validation
# ---------------------------------------------------------------------------


def validate_agent_install_preflight(profile: PlatformProfile, agent: AgentID) -> None:
    if agent == AgentID.KIMI:
        _validate_kimi_install_preflight(profile)


def _validate_kimi_install_preflight(profile: PlatformProfile) -> None:
    if not profile.supported:
        raise InstallError(
            f"Kimi is not supported on this platform ({profile.os}/{profile.linux_distro})"
        )

    if _cmd_look_path("uv") is None:
        raise InstallError(
            "Kimi requires Astral uv, but `uv` was not found in PATH.\n"
            f"Install uv and retry:\n  {_uv_install_hint(profile)}"
        )


def _uv_install_hint(profile: PlatformProfile) -> str:
    hints = {
        "brew": "brew install uv",
        "apt": "sudo apt-get install -y uv (or see https://docs.astral.sh/uv/getting-started/installation/)",
        "pacman": "sudo pacman -S --noconfirm uv",
        "dnf": "sudo dnf install -y uv",
        "winget": "winget install --id astral-sh.uv -e --accept-source-agreements --accept-package-agreements",
    }
    return hints.get(
        profile.package_manager,
        "https://docs.astral.sh/uv/getting-started/installation/",
    )


# ---------------------------------------------------------------------------
# Go preflight validation (Engram on non-brew platforms)
# ---------------------------------------------------------------------------


def _get_go_version_output() -> str:
    try:
        result = subprocess.run(
            ["go", "version"],
            capture_output=True,
            text=True,
            timeout=15,
        )
        return result.stdout.strip() if result.returncode == 0 else ""
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return ""


def validate_go_for_module_install(profile: PlatformProfile) -> None:
    if _cmd_look_path("go") is None:
        raise InstallError(
            "Go 1.24+ is required to install Dxrk Memory but was not found in PATH.\n"
            "Please install Go from https://go.dev/dl/ and restart your terminal."
        )

    out = _get_go_version_output()
    if not out:
        raise InstallError(
            "Go 1.24+ is required but could not verify the installed version.\n"
            "Please ensure Go is properly installed: https://go.dev/dl/"
        )

    parts = out.split()
    if len(parts) >= 3:
        version_str = parts[2].removeprefix("go")
        ver_parts = version_str.split(".", 2)
        if len(ver_parts) >= 2:
            try:
                major = int(ver_parts[0])
                minor = int(ver_parts[1])
            except ValueError:
                major = minor = 0
            if major < 1 or (major == 1 and minor < 24):
                raise InstallError(
                    f"Go 1.24+ is required to install Engram, but found go{version_str}.\n"
                    "Please update Go: https://go.dev/dl/"
                )

    if _os_getenv("GO111MODULE") == "off":
        fix = "export GO111MODULE=on  # then retry"
        if profile.os == "windows":
            fix = '$env:GO111MODULE = "on"  # PowerShell, then retry'
        raise InstallError(f"Go modules are disabled (GO111MODULE=off).\nRun: {fix}")


# ---------------------------------------------------------------------------
# Git Bash path resolution (Windows)
# ---------------------------------------------------------------------------


def _git_bash_path() -> str:
    git_path = _cmd_look_path("git")
    if git_path is not None:
        git_dir = os.path.dirname(git_path)
        parent = os.path.dirname(git_dir)

        candidate = os.path.join(parent, "bin", "bash.exe")
        if _file_exists(candidate):
            return candidate

        candidate = os.path.join(git_dir, "bash.exe")
        if _file_exists(candidate):
            return candidate

    candidates = [
        os.path.join(os.environ.get("ProgramFiles", ""), "Git", "bin", "bash.exe"),
        os.path.join(os.environ.get("ProgramFiles(x86)", ""), "Git", "bin", "bash.exe"),
        r"C:\Program Files\Git\bin\bash.exe",
    ]

    for c in candidates:
        if c and _file_exists(c):
            return c

    return "bash"


def git_bash_path() -> str:
    return _git_bash_path()


def _file_exists(path: str) -> bool:
    try:
        return _os_stat(path) is not None
    except OSError:
        return False


def _bash_script_path(profile: PlatformProfile, path: str) -> str:
    if profile.os == "windows":
        return path.replace("\\", "/")
    return path
