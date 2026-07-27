# SPDX-License-Identifier: MIT
from __future__ import annotations

import os
import subprocess

from dxrk.system import (
    _detect_from_inputs,
    _detect_linux_distro,
    _detect_single_dep,
    _install_hint_brew,
    _install_hint_curl,
    _install_hint_git,
    _install_hint_go,
    _install_hint_node,
    _install_hint_npm,
    _parse_version,
    _resolve_platform_profile,
    _version_at_least,
    _version_parts,
    add_to_user_path,
    detect_dependencies,
    detect_tools,
    ensure_supported_os,
    ensure_supported_platform,
    format_missing_deps_message,
    install_commands_for_dep,
    is_supported_os,
    render_dependency_report,
    scan_configs,
    ConfigState,
    Dependency,
    DependencyReport,
    DetectionResult,
    LINUX_DISTRO_ARCH,
    LINUX_DISTRO_DEBIAN,
    LINUX_DISTRO_FEDORA,
    LINUX_DISTRO_UNKNOWN,
    LINUX_DISTRO_UBUNTU,
    PlatformProfile,
    ToolStatus,
)

import pytest


class TestIsSupportedOS:
    def test_darwin(self):
        assert is_supported_os("darwin") is True

    def test_linux(self):
        assert is_supported_os("linux") is True

    def test_windows(self):
        assert is_supported_os("windows") is True

    def test_unsupported(self):
        assert is_supported_os("freebsd") is False


class TestDetectLinuxDistro:
    def test_ubuntu(self):
        content = 'ID=ubuntu\nVERSION_ID="22.04"\n'
        assert _detect_linux_distro(content) == LINUX_DISTRO_UBUNTU

    def test_debian(self):
        content = 'ID=debian\nVERSION_ID="12"\n'
        assert _detect_linux_distro(content) == LINUX_DISTRO_DEBIAN

    def test_popos_like_ubuntu(self):
        content = 'ID=pop\nID_LIKE="ubuntu"\n'
        assert _detect_linux_distro(content) == LINUX_DISTRO_UBUNTU

    def test_mint_like_debian(self):
        content = 'ID=linuxmint\nID_LIKE="ubuntu"\n'
        assert _detect_linux_distro(content) == LINUX_DISTRO_UBUNTU

    def test_arch(self):
        content = "ID=arch\n"
        assert _detect_linux_distro(content) == LINUX_DISTRO_ARCH

    def test_fedora(self):
        content = 'ID=fedora\nVERSION_ID="40"\n'
        assert _detect_linux_distro(content) == LINUX_DISTRO_FEDORA

    def test_rhel_like_fedora(self):
        content = 'ID=rhel\nID_LIKE="fedora"\n'
        assert _detect_linux_distro(content) == LINUX_DISTRO_FEDORA

    def test_empty_content(self):
        assert _detect_linux_distro("") == LINUX_DISTRO_UNKNOWN

    def test_unsupported_distro(self):
        content = 'ID=alpine\n'
        assert _detect_linux_distro(content) == LINUX_DISTRO_UNKNOWN


class TestResolvePlatformProfile:
    def test_darwin(self):
        profile = _resolve_platform_profile("darwin", "", {})
        assert profile.os == "darwin"
        assert profile.package_manager == "brew"
        assert profile.supported is True

    def test_linux_with_brew(self):
        tools = {"brew": ToolStatus(name="brew", installed=True, path="/home/linuxbrew/.linuxbrew/bin/brew")}
        profile = _resolve_platform_profile("linux", 'ID=ubuntu\n', tools)
        assert profile.os == "linux"
        assert profile.package_manager == "brew"
        assert profile.supported is True

    def test_linux_ubuntu(self):
        profile = _resolve_platform_profile("linux", 'ID=ubuntu\n', {})
        assert profile.linux_distro == LINUX_DISTRO_UBUNTU
        assert profile.package_manager == "apt"
        assert profile.supported is True

    def test_linux_arch(self):
        profile = _resolve_platform_profile("linux", 'ID=arch\n', {})
        assert profile.linux_distro == LINUX_DISTRO_ARCH
        assert profile.package_manager == "pacman"
        assert profile.supported is True

    def test_linux_fedora(self):
        profile = _resolve_platform_profile("linux", 'ID=fedora\n', {})
        assert profile.linux_distro == LINUX_DISTRO_FEDORA
        assert profile.package_manager == "dnf"
        assert profile.supported is True

    def test_linux_unsupported(self):
        profile = _resolve_platform_profile("linux", 'ID=alpine\n', {})
        assert profile.linux_distro == LINUX_DISTRO_UNKNOWN
        assert profile.package_manager == ""
        assert profile.supported is False

    def test_windows(self):
        profile = _resolve_platform_profile("windows", "", {})
        assert profile.os == "windows"
        assert profile.package_manager == "winget"
        assert profile.supported is True


class TestDetectFromInputs:
    def test_darwin_detection(self):
        result = _detect_from_inputs("darwin", "arm64", "/bin/zsh", "", {}, [])
        assert isinstance(result, DetectionResult)
        assert result.system.os == "darwin"
        assert result.system.arch == "arm64"
        assert result.system.shell == "/bin/zsh"
        assert result.system.supported is True
        assert result.system.profile.package_manager == "brew"

    def test_linux_supported(self):
        os_release = 'ID=ubuntu\n'
        tools = {}
        result = _detect_from_inputs("linux", "x86_64", "/bin/bash", os_release, tools, [])
        assert result.system.os == "linux"
        assert result.system.supported is True
        assert result.system.profile.package_manager == "apt"

    def test_windows_default_shell(self):
        result = _detect_from_inputs("windows", "amd64", "", "", {}, [])
        assert result.system.shell == "powershell"

    def test_unknown_os(self):
        result = _detect_from_inputs("freebsd", "amd64", "/bin/sh", "", {}, [])
        assert result.system.supported is False


class TestDetectTools:
    def test_returns_tool_status_dict(self):
        tools = detect_tools(["git", "this_tool_does_not_exist_xyz"])
        assert isinstance(tools, dict)
        assert "git" in tools
        assert isinstance(tools["git"], ToolStatus)

    def test_installed_tool(self):
        tools = detect_tools(["python3"])
        assert tools["python3"].installed is True
        assert tools["python3"].path != ""

    def test_missing_tool(self):
        tools = detect_tools(["zzz_nonexistent_tool_999"])
        assert tools["zzz_nonexistent_tool_999"].installed is False
        assert tools["zzz_nonexistent_tool_999"].path == ""


class TestScanConfigs:
    def test_returns_all_config_roots(self, tmp_path):
        states = scan_configs(str(tmp_path))
        assert len(states) == 12
        assert all(isinstance(s, ConfigState) for s in states)
        assert states[0].agent == "claude-code"

    def test_detects_existing_directory(self, tmp_path):
        (tmp_path / ".config" / "opencode").mkdir(parents=True)
        states = scan_configs(str(tmp_path))
        opencode = next(s for s in states if s.agent == "opencode")
        assert opencode.exists is True
        assert opencode.is_directory is True
        claude = next(s for s in states if s.agent == "claude-code")
        assert claude.exists is False

    def test_ignores_non_config_file(self, tmp_path):
        (tmp_path / ".gitkeep").write_text("")
        states = scan_configs(str(tmp_path))
        assert all(s.exists is False for s in states)

    def test_non_existent_path(self):
        states = scan_configs("/tmp/zzz_nonexistent_path_abc123")
        assert all(s.exists is False for s in states)


class TestParseVersion:
    def test_git_version(self):
        assert _parse_version("git", "git version 2.43.0\n") == "2.43.0"

    def test_node_version_with_v(self):
        assert _parse_version("node", "v20.11.0\n") == "20.11.0"

    def test_go_version(self):
        assert _parse_version("go", "go version go1.22.0 linux/amd64\n") == "1.22.0"

    def test_go_without_v(self):
        assert _parse_version("go", "go1.21.5\n") == "1.21.5"

    def test_curl_version(self):
        assert _parse_version("curl", "curl 8.4.0 (x86_64-linux-gnu)\n") == "8.4.0"

    def test_empty_output(self):
        assert _parse_version("git", "") == ""

    def test_non_matching_output(self):
        assert _parse_version("git", "not a version string\n") == ""


class TestVersionParts:
    def test_full_semver(self):
        assert _version_parts("2.43.0") == [2, 43, 0]

    def test_major_minor(self):
        assert _version_parts("20.11") == [20, 11, 0]

    def test_major_only(self):
        assert _version_parts("3") == [3, 0, 0]

    def test_empty(self):
        assert _version_parts("") == [0, 0, 0]

    def test_non_numeric(self):
        assert _version_parts("abc") == [0, 0, 0]


class TestVersionAtLeast:
    def test_equal(self):
        assert _version_at_least("18.0.0", "18.0.0") is True

    def test_newer_major(self):
        assert _version_at_least("20.0.0", "18.0.0") is True

    def test_newer_minor(self):
        assert _version_at_least("18.5.0", "18.0.0") is True

    def test_newer_patch(self):
        assert _version_at_least("18.0.1", "18.0.0") is True

    def test_older_major(self):
        assert _version_at_least("16.0.0", "18.0.0") is False

    def test_older_minor(self):
        assert _version_at_least("18.0.0", "18.5.0") is False

    def test_shorter_version(self):
        assert _version_at_least("18", "18.0.0") is True


class TestDetectSingleDep:
    def test_no_detect_cmd(self):
        dep = Dependency(name="test")
        result = _detect_single_dep(dep)
        assert result.installed is False

    def test_binary_not_found(self, monkeypatch):
        monkeypatch.setattr("shutil.which", lambda _: None)
        dep = Dependency(name="git", detect_cmd=["git", "--version"])
        result = _detect_single_dep(dep)
        assert result.installed is False

    def test_detects_installed(self, monkeypatch):
        monkeypatch.setattr("shutil.which", lambda _: "/usr/bin/git")
        result = subprocess.run(["python3", "--version"], capture_output=True, text=True)
        monkeypatch.setattr("subprocess.run", lambda *a, **kw: result)
        dep = Dependency(name="python3", detect_cmd=["python3", "--version"])
        result_dep = _detect_single_dep(dep)
        assert result_dep.installed is True
        assert result_dep.version != ""

    def test_version_too_low(self, monkeypatch):
        monkeypatch.setattr("shutil.which", lambda _: "/usr/bin/node")

        class FakeResult:
            returncode = 0
            stdout = "v16.0.0\n"
            stderr = ""

        monkeypatch.setattr("subprocess.run", lambda *a, **kw: FakeResult(), raising=False)
        dep = Dependency(name="node", min_version="18.0.0", detect_cmd=["node", "--version"])
        result = _detect_single_dep(dep)
        assert result.installed is False


class TestFormatMissingDepsMessage:
    def test_all_present(self):
        report = DependencyReport(all_present=True)
        msg = format_missing_deps_message(report)
        assert msg == "All required dependencies are present."

    def test_missing_required(self):
        report = DependencyReport(
            all_present=False,
            missing_required=["git"],
            dependencies=[
                Dependency(name="git", required=True, installed=False, install_hint="brew install git"),
            ],
        )
        msg = format_missing_deps_message(report)
        assert "Missing" in msg
        assert "git" in msg
        assert "brew install git" in msg


class TestRenderDependencyReport:
    def test_installed(self):
        report = DependencyReport(
            dependencies=[
                Dependency(name="git", installed=True, version="2.43.0"),
            ],
        )
        out = render_dependency_report(report)
        assert "git" in out
        assert "v" in out
        assert "2.43.0" in out

    def test_missing_required(self):
        report = DependencyReport(
            all_present=False,
            missing_required=["git"],
            dependencies=[
                Dependency(name="git", required=True, installed=False),
            ],
        )
        out = render_dependency_report(report)
        assert "x" in out
        assert "NOT FOUND" in out
        assert "(required)" in out

    def test_missing_optional(self):
        report = DependencyReport(
            missing_optional=["brew"],
            dependencies=[
                Dependency(name="brew", required=False, installed=False),
            ],
        )
        out = render_dependency_report(report)
        assert "(optional)" in out


class TestInstallCommandsForDep:
    def test_git_darwin(self):
        profile = PlatformProfile(os="darwin", package_manager="brew")
        cmds = install_commands_for_dep("git", profile)
        assert cmds == [["brew", "install", "git"]]

    def test_git_linux_apt(self):
        profile = PlatformProfile(os="linux", package_manager="apt")
        cmds = install_commands_for_dep("git", profile)
        assert cmds == [["sudo", "apt-get", "install", "-y", "git"]]

    def test_curl_windows_returns_none(self):
        profile = PlatformProfile(os="windows", package_manager="winget")
        cmds = install_commands_for_dep("curl", profile)
        assert cmds is None

    def test_npm_always_none(self):
        profile = PlatformProfile(os="darwin", package_manager="brew")
        cmds = install_commands_for_dep("npm", profile)
        assert cmds is None

    def test_brew_only_darwin(self):
        profile = PlatformProfile(os="linux", package_manager="brew")
        cmds = install_commands_for_dep("brew", profile)
        assert cmds is None

    def test_unknown_dep(self):
        profile = PlatformProfile(os="darwin", package_manager="brew")
        cmds = install_commands_for_dep("zzz_nonexistent", profile)
        assert cmds is None


class TestEnsureFunctions:
    def test_supported_os_ok(self):
        ensure_supported_os("darwin")
        ensure_supported_os("linux")
        ensure_supported_os("windows")

    def test_unsupported_os_raises(self):
        with pytest.raises(OSError, match="unsupported operating system"):
            ensure_supported_os("freebsd")

    def test_unsupported_linux_distro_raises(self):
        profile = PlatformProfile(os="linux", linux_distro=LINUX_DISTRO_UNKNOWN, supported=False)
        with pytest.raises(OSError, match="unsupported linux distro"):
            ensure_supported_platform(profile)


class TestInstallHints:
    def test_git_hints(self):
        assert "brew install git" in _install_hint_git(PlatformProfile(os="darwin"))
        assert "winget" in _install_hint_git(PlatformProfile(os="windows"))
        assert "apt" in _install_hint_git(PlatformProfile(os="linux", package_manager="apt"))
        assert "pacman" in _install_hint_git(PlatformProfile(os="linux", package_manager="pacman"))
        assert "dnf" in _install_hint_git(PlatformProfile(os="linux", package_manager="dnf"))

    def test_curl_hints(self):
        assert "brew" in _install_hint_curl(PlatformProfile(os="darwin"))
        assert "pre-installed" in _install_hint_curl(PlatformProfile(os="windows"))
        assert "apt" in _install_hint_curl(PlatformProfile(os="linux", package_manager="apt"))

    def test_node_hints(self):
        assert "brew" in _install_hint_node(PlatformProfile(os="darwin"))
        assert "winget" in _install_hint_node(PlatformProfile(os="windows"))
        assert "deb.nodesource" in _install_hint_node(PlatformProfile(os="linux", package_manager="apt"))

    def test_npm_hint(self):
        assert "install node first" in _install_hint_npm(PlatformProfile(os="darwin"))

    def test_brew_hint(self):
        assert "Homebrew/install" in _install_hint_brew()

    def test_go_hints(self):
        assert "brew" in _install_hint_go(PlatformProfile(os="darwin"))
        assert "winget" in _install_hint_go(PlatformProfile(os="windows"))
        assert "apt" in _install_hint_go(PlatformProfile(os="linux", package_manager="apt"))


class TestAddToUserPath:
    def test_adds_to_path_on_unix(self, monkeypatch):
        called_path = None

        def fake_add(dir_: str) -> None:
            nonlocal called_path
            called_path = dir_

        monkeypatch.setattr("dxrk.system._add_to_process_path", fake_add)
        add_to_user_path("/custom/bin")
        assert called_path == "/custom/bin"

    def test_skips_duplicate_path(self, monkeypatch):
        monkeypatch.setattr("dxrk.system.os.environ", {"PATH": "/usr/bin:/custom/bin"})
        monkeypatch.setattr("dxrk.system._add_to_process_path", lambda _: None)
        add_to_user_path("/custom/bin")
        assert "PATH" in os.environ


class TestDetectDependencies:
    def test_detect_dependencies_darwin(self, monkeypatch):
        profile = PlatformProfile(os="darwin", package_manager="brew", supported=True)

        def fake_detect_single(dep):
            dep.installed = True
            dep.version = "1.0.0"
            return dep

        monkeypatch.setattr("dxrk.system._detect_single_dep", fake_detect_single)

        report = detect_dependencies(profile)
        assert report.all_present is True
        assert len(report.dependencies) == 6  # git, curl, node, npm, brew, go

    def test_defines_brew_for_darwin(self):
        profile = PlatformProfile(os="darwin", package_manager="brew")
        from dxrk.system import _define_dependencies
        deps = _define_dependencies(profile)
        names = [d.name for d in deps]
        assert "brew" in names

    def test_no_brew_for_linux(self):
        profile = PlatformProfile(os="linux", package_manager="apt")
        from dxrk.system import _define_dependencies
        deps = _define_dependencies(profile)
        names = [d.name for d in deps]
        assert "brew" not in names
