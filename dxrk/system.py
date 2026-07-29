# SPDX-License-Identifier: MIT
from __future__ import annotations
import os
import platform
import re
import shutil
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from functools import lru_cache
from typing import Optional

LINUX_DISTRO_UNKNOWN = "unknown"
LINUX_DISTRO_UBUNTU = "ubuntu"
LINUX_DISTRO_DEBIAN = "debian"
LINUX_DISTRO_ARCH = "arch"
LINUX_DISTRO_FEDORA = "fedora"


@dataclass
class ToolStatus:
    name: str = ""
    installed: bool = False
    path: str = ""


@dataclass
class ConfigState:
    agent: str = ""
    path: str = ""
    exists: bool = False
    is_directory: bool = False


@dataclass
class PlatformProfile:
    os: str = ""
    linux_distro: str = ""
    package_manager: str = ""
    npm_writable: bool = False
    supported: bool = False


@dataclass
class SystemInfo:
    os: str = ""
    arch: str = ""
    shell: str = ""
    supported: bool = False
    profile: PlatformProfile = field(default_factory=PlatformProfile)


@dataclass
class Dependency:
    name: str = ""
    required: bool = False
    min_version: str = ""
    detect_cmd: list[str] = field(default_factory=list)
    installed: bool = False
    version: str = ""
    install_hint: str = ""


@dataclass
class DependencyReport:
    dependencies: list[Dependency] = field(default_factory=list)
    all_present: bool = True
    missing_required: list[str] = field(default_factory=list)
    missing_optional: list[str] = field(default_factory=list)


@dataclass
class DetectionResult:
    system: SystemInfo = field(default_factory=SystemInfo)
    tools: dict[str, ToolStatus] = field(default_factory=dict)
    configs: list[ConfigState] = field(default_factory=list)
    dependencies: DependencyReport = field(default_factory=DependencyReport)


ERROR_UNSUPPORTED_OS = "unsupported operating system"
ERROR_UNSUPPORTED_LINUX_DISTRO = "unsupported linux distro"


_DEFAULT_CONFIG_ROOTS: list[tuple[str, str]] = [
    ("claude-code", ".claude"),
    ("opencode", ".config/opencode"),
    ("kilocode", ".config/kilo"),
    ("gemini-cli", ".gemini"),
    ("cursor", ".cursor"),
    ("vscode-copilot", ".copilot"),
    ("codex", ".codex"),
    ("antigravity", ".gemini/antigravity"),
    ("windsurf", ".codeium/windsurf"),
    ("kimi", ".kimi"),
    ("qwen-code", ".qwen"),
    ("kiro-ide", ".kiro"),
]

_VERSION_REGEX = re.compile(r"(\d+\.\d+(?:\.\d+)?)")
_GO_VERSION_REGEX = re.compile(r"go(\d+\.\d+(?:\.\d+)?)")


def is_supported_os(goos: str) -> bool:
    return goos in ("darwin", "linux", "windows")


def detect_tools(names: list[str]) -> dict[str, ToolStatus]:
    tools: dict[str, ToolStatus] = {}
    for name in names:
        path = shutil.which(name)
        tools[name] = ToolStatus(
            name=name,
            installed=path is not None,
            path=path or "",
        )
    return tools


def _detect_npm_writable(home_dir: str) -> bool:
    try:
        result = subprocess.run(
            ["npm", "config", "get", "prefix"],
            capture_output=True, text=True, timeout=10,
        )
        if result.returncode != 0:
            return False
        prefix = result.stdout.strip()
        return prefix.startswith(home_dir)
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return False


def _os_release_content() -> str:
    try:
        with open("/etc/os-release") as f:
            return f.read()
    except FileNotFoundError:
        return ""


def _detect_linux_distro(content: str) -> str:
    content = content.strip()
    if not content:
        return LINUX_DISTRO_UNKNOWN

    fields: dict[str, str] = {}
    for line in content.split("\n"):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split("=", 1)
        if len(parts) != 2:
            continue
        key = parts[0].strip().upper()
        value = parts[1].strip().strip('"').lower()
        fields[key] = value

    distro_id = fields.get("ID", "")
    distro_id_like = fields.get("ID_LIKE", "")

    if distro_id in (LINUX_DISTRO_UBUNTU, LINUX_DISTRO_DEBIAN) or any(
        t in (LINUX_DISTRO_UBUNTU, LINUX_DISTRO_DEBIAN) for t in distro_id_like.split()
    ):
        return LINUX_DISTRO_DEBIAN if distro_id == LINUX_DISTRO_DEBIAN else LINUX_DISTRO_UBUNTU

    if distro_id == LINUX_DISTRO_ARCH or LINUX_DISTRO_ARCH in distro_id_like.split():
        return LINUX_DISTRO_ARCH

    fedora_family = {LINUX_DISTRO_FEDORA, "rhel", "centos", "rocky", "almalinux", "nobara"}
    if distro_id in fedora_family or fedora_family & set(distro_id_like.split()):
        return LINUX_DISTRO_FEDORA

    return LINUX_DISTRO_UNKNOWN


def _resolve_platform_profile(goos: str, linux_release: str, tools: dict[str, ToolStatus]) -> PlatformProfile:
    profile = PlatformProfile(os=goos)

    if goos == "darwin":
        profile.package_manager = "brew"
        profile.supported = True
        return profile

    if goos == "linux":
        distro = _detect_linux_distro(linux_release)
        profile.linux_distro = distro

        brew_tool = tools.get("brew")
        if brew_tool and brew_tool.installed:
            profile.package_manager = "brew"
            profile.supported = True
            return profile

        if distro in (LINUX_DISTRO_UBUNTU, LINUX_DISTRO_DEBIAN):
            profile.package_manager = "apt"
            profile.supported = True
        elif distro == LINUX_DISTRO_ARCH:
            profile.package_manager = "pacman"
            profile.supported = True
        elif distro == LINUX_DISTRO_FEDORA:
            profile.package_manager = "dnf"
            profile.supported = True

        return profile

    if goos == "windows":
        profile.package_manager = "winget"
        profile.supported = True
        return profile

    return profile


def _detect_from_inputs(goos: str, arch: str, shell: str, linux_release: str, tools: dict[str, ToolStatus], configs: list[ConfigState]) -> DetectionResult:
    if not shell:
        shell = "powershell" if goos == "windows" else "unknown"

    profile = _resolve_platform_profile(goos, linux_release, tools)

    return DetectionResult(
        system=SystemInfo(
            os=goos,
            arch=arch,
            shell=shell,
            supported=profile.supported,
            profile=profile,
        ),
        tools=tools,
        configs=configs,
    )


def scan_configs(home_dir: str) -> list[ConfigState]:
    states: list[ConfigState] = []
    for agent, rel_path in _DEFAULT_CONFIG_ROOTS:
        full_path = os.path.join(home_dir, rel_path)
        config = ConfigState(agent=agent, path=full_path)
        try:
            stat = os.stat(full_path)
            config.exists = True
            config.is_directory = (stat.st_mode & 0o0040000) != 0
        except FileNotFoundError:
            pass
        states.append(config)
    return states


def detect() -> DetectionResult:
    home = os.path.expanduser("~")
    goos = sys.platform
    if goos == "darwin":
        goos = "darwin"
    elif goos.startswith("linux"):
        goos = "linux"
    elif goos.startswith("win"):
        goos = "windows"

    arch = platform.machine()
    shell_env = os.environ.get("SHELL", "")

    tools = detect_tools(["git", "curl", "brew", "node"])
    configs = scan_configs(home)
    os_release = _os_release_content()

    result = _detect_from_inputs(goos, arch, shell_env, os_release, tools, configs)

    if goos == "windows":
        result.system.profile.npm_writable = True
    else:
        result.system.profile.npm_writable = _detect_npm_writable(home)

    result.dependencies = detect_dependencies(result.system.profile)

    return result


def ensure_supported_os(goos: str) -> None:
    if not is_supported_os(goos):
        raise OSError(f"{ERROR_UNSUPPORTED_OS}: only macOS, Linux, and Windows are supported (detected {goos})")


def ensure_supported_platform(profile: PlatformProfile) -> None:
    ensure_supported_os(profile.os)
    if profile.os == "linux" and not profile.supported:
        raise OSError(
            f"{ERROR_UNSUPPORTED_LINUX_DISTRO}: Linux support is limited to "
            f"Ubuntu/Debian, Arch, and Fedora/RHEL family (detected {profile.linux_distro})"
        )


def _install_hint_git(profile: PlatformProfile) -> str:
    if profile.os == "darwin":
        return "brew install git"
    if profile.os == "windows":
        return "winget install Git.Git"
    if profile.package_manager == "apt":
        return "sudo apt-get install -y git"
    if profile.package_manager == "pacman":
        return "sudo pacman -S --noconfirm git"
    if profile.package_manager == "dnf":
        return "sudo dnf install -y git"
    return "install git from https://git-scm.com/"


def _install_hint_curl(profile: PlatformProfile) -> str:
    if profile.os == "darwin":
        return "brew install curl"
    if profile.os == "windows":
        return "curl is pre-installed on Windows 10+"
    if profile.package_manager == "apt":
        return "sudo apt-get install -y curl"
    if profile.package_manager == "pacman":
        return "sudo pacman -S --noconfirm curl"
    if profile.package_manager == "dnf":
        return "sudo dnf install -y curl"
    return "install curl from https://curl.se/"


def _install_hint_node(profile: PlatformProfile) -> str:
    if profile.os == "darwin":
        return "brew install node"
    if profile.os == "windows":
        return "winget install OpenJS.NodeJS.LTS"
    if profile.package_manager == "apt":
        return "curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash - && sudo apt-get install -y nodejs"
    if profile.package_manager == "pacman":
        return "sudo pacman -S --noconfirm nodejs npm"
    if profile.package_manager == "dnf":
        return "curl -fsSL https://rpm.nodesource.com/setup_lts.x | sudo bash - && sudo dnf install -y nodejs"
    return "install node from https://nodejs.org/"


def _install_hint_npm(_profile: PlatformProfile) -> str:
    return "npm is included with node -- install node first"


def _install_hint_brew() -> str:
    return '/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"'


def _install_hint_go(profile: PlatformProfile) -> str:
    if profile.os == "darwin":
        return "brew install go"
    if profile.os == "windows":
        return "winget install GoLang.Go"
    if profile.package_manager == "apt":
        return "sudo apt-get install -y golang"
    if profile.package_manager == "pacman":
        return "sudo pacman -S --noconfirm go"
    if profile.package_manager == "dnf":
        return "sudo dnf install -y golang"
    return "install go from https://go.dev/dl/"


def _define_dependencies(profile: PlatformProfile) -> list[Dependency]:
    deps: list[Dependency] = [
        Dependency(name="git", required=True, detect_cmd=["git", "--version"], install_hint=_install_hint_git(profile)),
        Dependency(name="curl", required=True, detect_cmd=["curl", "--version"], install_hint=_install_hint_curl(profile)),
        Dependency(name="node", required=True, min_version="18.0.0", detect_cmd=["node", "--version"], install_hint=_install_hint_node(profile)),
        Dependency(name="npm", required=True, detect_cmd=["npm", "--version"], install_hint=_install_hint_npm(profile)),
    ]

    if profile.os == "darwin":
        deps.append(Dependency(name="brew", required=False, detect_cmd=["brew", "--version"], install_hint=_install_hint_brew()))

    deps.append(Dependency(name="go", required=False, detect_cmd=["go", "version"], install_hint=_install_hint_go(profile)))

    return deps


def _parse_version(name: str, output: str) -> str:
    output = output.strip()
    if not output:
        return ""

    if name == "go":
        m = _GO_VERSION_REGEX.search(output)
        if m:
            return m.group(1)

    m = _VERSION_REGEX.search(output)
    return m.group(1) if m else ""


def _version_parts(version: str) -> list[int]:
    parts = version.split(".")
    result: list[int] = []
    for i in range(3):
        if i < len(parts):
            try:
                result.append(int(parts[i]))
            except ValueError:
                result.append(0)
        else:
            result.append(0)
    return result


def _version_at_least(version: str, min_version: str) -> bool:
    v = _version_parts(version)
    m = _version_parts(min_version)
    for i in range(3):
        if v[i] > m[i]:
            return True
        if v[i] < m[i]:
            return False
    return True


def _detect_single_dep(dep: Dependency) -> Dependency:
    if not dep.detect_cmd:
        return dep

    binary = dep.detect_cmd[0]
    if not shutil.which(binary):
        return dep

    dep.installed = True

    try:
        result = subprocess.run(
            dep.detect_cmd,
            capture_output=True, text=True, timeout=15,
        )
        if result.returncode == 0:
            dep.version = _parse_version(dep.name, result.stdout)
    except (FileNotFoundError, subprocess.TimeoutExpired):
        pass

    if dep.min_version and dep.version:
        if not _version_at_least(dep.version, dep.min_version):
            dep.installed = False

    return dep


def detect_dependencies(profile: PlatformProfile) -> DependencyReport:
    deps = _define_dependencies(profile)

    results: list[Dependency] = [None] * len(deps)  # type: ignore

    with ThreadPoolExecutor(max_workers=8) as pool:
        futures = {pool.submit(_detect_single_dep, dep): i for i, dep in enumerate(deps)}
        for future in as_completed(futures):
            idx = futures[future]
            results[idx] = future.result()

    report = DependencyReport(dependencies=results, all_present=True)

    for dep in results:
        if dep.required and not dep.installed:
            report.all_present = False
            report.missing_required.append(dep.name)
        elif not dep.required and not dep.installed:
            report.missing_optional.append(dep.name)

    return report


def install_commands_for_dep(name: str, profile: PlatformProfile) -> Optional[list[list[str]]]:
    commands: dict[str, dict[str, list[list[str]]]] = {
        "git": {
            "darwin": [["brew", "install", "git"]],
            "windows": [["winget", "install", "--id", "Git.Git", "-e", "--accept-source-agreements", "--accept-package-agreements"]],
            "apt": [["sudo", "apt-get", "install", "-y", "git"]],
            "pacman": [["sudo", "pacman", "-S", "--noconfirm", "git"]],
            "dnf": [["sudo", "dnf", "install", "-y", "git"]],
        },
        "curl": {
            "darwin": [["brew", "install", "curl"]],
            "apt": [["sudo", "apt-get", "install", "-y", "curl"]],
            "pacman": [["sudo", "pacman", "-S", "--noconfirm", "curl"]],
            "dnf": [["sudo", "dnf", "install", "-y", "curl"]],
        },
        "node": {
            "darwin": [["brew", "install", "node"]],
            "windows": [["winget", "install", "--id", "OpenJS.NodeJS.LTS", "-e", "--accept-source-agreements", "--accept-package-agreements"]],
            "apt": [["bash", "-c", "curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -"], ["sudo", "apt-get", "install", "-y", "nodejs"]],
            "pacman": [["sudo", "pacman", "-S", "--noconfirm", "nodejs", "npm"]],
            "dnf": [["bash", "-c", "curl -fsSL https://rpm.nodesource.com/setup_lts.x | sudo bash -"], ["sudo", "dnf", "install", "-y", "nodejs"]],
        },
        "brew": {
            "darwin": [["bash", "-c", "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"]],
        },
        "go": {
            "darwin": [["brew", "install", "go"]],
            "windows": [["winget", "install", "--id", "GoLang.Go", "-e", "--accept-source-agreements", "--accept-package-agreements"]],
            "apt": [["sudo", "apt-get", "install", "-y", "golang"]],
            "pacman": [["sudo", "pacman", "-S", "--noconfirm", "go"]],
            "dnf": [["sudo", "dnf", "install", "-y", "golang"]],
        },
    }

    dep_commands = commands.get(name)
    if not dep_commands:
        return None

    if name == "curl" and profile.os == "windows":
        return None
    if name == "npm":
        return None

    if name == "brew" and profile.os != "darwin":
        return None

    if name in ("node", "git", "curl", "go"):
        # Linux uses package_manager as key (apt/pacman/dnf),
        # macOS/Windows use OS name as key (darwin/windows)
        if profile.os in ("darwin", "windows"):
            pm = profile.os
        else:
            pm = profile.package_manager
    else:
        pm = profile.os
    return dep_commands.get(pm) or dep_commands.get(profile.package_manager)


def format_missing_deps_message(report: DependencyReport) -> str:
    if report.all_present:
        return "All required dependencies are present."

    msg = f"Missing {len(report.missing_required)} required dependenc(ies): {', '.join(report.missing_required)}\n"
    msg += "\nInstall hints:\n"
    for dep in report.dependencies:
        if not dep.installed and dep.required:
            msg += f"  {dep.name}: {dep.install_hint}\n"

    return msg


def render_dependency_report(report: DependencyReport) -> str:
    lines = ["Dependencies:"]
    for dep in report.dependencies:
        if dep.installed:
            marker = "v"
            status = dep.version or "found"
        else:
            marker = "x"
            status = "NOT FOUND"

        suffix = ""
        if not dep.installed and dep.required:
            suffix = " (required)"
        elif not dep.required:
            suffix = " (optional)"

        lines.append(f"  {dep.name}: {marker} {status}{suffix}")

    if report.missing_required:
        lines.append(f"Missing required: {', '.join(report.missing_required)}")
    if report.missing_optional:
        lines.append(f"Missing optional: {', '.join(report.missing_optional)}")

    return "\n".join(lines)


def add_to_user_path(directory: str) -> None:
    if sys.platform != "win32":
        _add_to_process_path(directory)
        return

    current = os.environ.get("PATH", "")
    normalized = os.path.normpath(directory)
    for p in current.split(os.pathsep):
        if os.path.normpath(p).lower() == normalized.lower():
            return

    _add_to_process_path(directory)

    safe_dir = directory.replace("'", "''")
    script = (
        f"$current = [Environment]::GetEnvironmentVariable('PATH', 'User'); "
        f"if (($current.Split(';')) -notcontains '{safe_dir}') {{ "
        f"[Environment]::SetEnvironmentVariable('PATH', '{safe_dir};' + $current, 'User') }}"
    )
    subprocess.run(
        ["powershell", "-NoProfile", "-NonInteractive", "-Command", script],
        capture_output=True, timeout=30,
    )


def _add_to_process_path(directory: str) -> None:
    current = os.environ.get("PATH", "")
    normalized = os.path.normpath(directory)
    for p in current.split(os.pathsep):
        if os.path.normpath(p).lower() == normalized.lower():
            return
    if current:
        os.environ["PATH"] = f"{directory}{os.pathsep}{current}"
    else:
        os.environ["PATH"] = directory
