from __future__ import annotations

import json
import os
import pathlib
import subprocess
import tarfile
import tempfile
import unittest


REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parents[1]
RELEASE_ROOT = REPOSITORY_ROOT / "scripts" / "release"
PREPARE_RELEASE = RELEASE_ROOT / "prepare_release.sh"
PREPARE_PAGES = RELEASE_ROOT / "prepare_pages_artifact.sh"
DEPLOY_PAGES = RELEASE_ROOT / "deploy_pages_artifact.sh"
RELEASE_HELPER = RELEASE_ROOT / "release_helper.py"
RELEASE_VERSION = "v1.2.0"


class PagesReleaseContractTest(unittest.TestCase):
    def command(
        self,
        *command: str,
        cwd: pathlib.Path,
        env: dict[str, str] | None = None,
        check: bool = True,
        git_dir: bool = False,
    ) -> subprocess.CompletedProcess[str]:
        actual_command = list(command)
        if git_dir:
            actual_command = [actual_command[0], f"--git-dir={cwd}", *actual_command[1:]]
            cwd = self.root
        return subprocess.run(
            actual_command,
            cwd=cwd,
            env=env,
            check=check,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

    def setUp(self) -> None:
        self.temporary_directory = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.temporary_directory.name)
        self.remote = self.root / "origin.git"
        self.repo = self.root / "repo"
        self.command("git", "init", "--bare", str(self.remote), cwd=self.root)
        self.command("git", "clone", str(self.remote), str(self.repo), cwd=self.root)
        self.command("git", "config", "user.name", "Release Contract Test", cwd=self.repo)
        self.command("git", "config", "user.email", "release-contract@example.invalid", cwd=self.repo)
        (self.repo / "site").mkdir()
        (self.repo / "site" / "index.html").write_text("<!doctype html><title>Fixture</title>\n", encoding="utf-8")
        (self.repo / "Makefile").write_text(
            "ci:\n\t@true\n\n"
            "pages-artifact:\n"
            f"\t@\"{PREPARE_PAGES}\" --source site\n",
            encoding="utf-8",
        )
        self.command("git", "add", "Makefile", "site", cwd=self.repo)
        self.command("git", "commit", "-m", "Initial Pages source", cwd=self.repo)
        self.command("git", "branch", "-M", "master", cwd=self.repo)
        self.command("git", "push", "-u", "origin", "master", cwd=self.repo)
        self.command("git", "symbolic-ref", "HEAD", "refs/heads/master", cwd=self.remote, git_dir=True)
        self.command("git", "remote", "set-head", "origin", "-a", cwd=self.repo)

    def tearDown(self) -> None:
        self.temporary_directory.cleanup()

    def test_pages_release_preserves_commit_roles_and_marker_visibility(self) -> None:
        environment = os.environ.copy()
        environment["GH_REPO"] = "example/release-contract"
        environment["RELEASE_HELPER"] = str(RELEASE_HELPER)
        environment["RELEASE_ARTIFACT_TARGETS"] = "pages-artifact"
        self.command(str(PREPARE_RELEASE), "--version", RELEASE_VERSION, cwd=self.repo, env=environment)

        source_commit = self.command("git", "rev-parse", "HEAD^", cwd=self.repo).stdout.strip()
        release_commit = self.command("git", "rev-parse", "HEAD", cwd=self.repo).stdout.strip()
        self.assertNotEqual(source_commit, release_commit)
        artifact_directory = pathlib.Path(
            self.command("git", "rev-parse", "--git-path", "mprlab-release", cwd=self.repo).stdout.strip()
        )
        if not artifact_directory.is_absolute():
            artifact_directory = self.repo / artifact_directory
        manifest = json.loads((artifact_directory / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(manifest["source_commit"], source_commit)
        self.assertEqual(manifest["release_commit"], release_commit)

        archive_path = artifact_directory / "payloads" / "release-assets" / "pages.tar.gz"
        with tarfile.open(archive_path, "r:gz") as archive:
            members = {member.name.removeprefix("./"): member for member in archive.getmembers()}
            self.assertEqual(members[".nojekyll"].size, 0)
            marker_file = archive.extractfile(members[".mprlab-release.json"])
            self.assertIsNotNone(marker_file)
            marker = json.load(marker_file)
        self.assertEqual(marker["schema_version"], 1)
        self.assertEqual(marker["release_version"], RELEASE_VERSION)
        self.assertEqual(marker["source_commit"], source_commit)

        self.command("git", "push", "origin", "HEAD:refs/heads/master", cwd=self.repo)
        self.command("git", "push", "origin", f"refs/tags/{RELEASE_VERSION}:refs/tags/{RELEASE_VERSION}", cwd=self.repo)
        public_marker = self.root / "public-marker.json"
        public_marker.write_text(json.dumps(marker), encoding="utf-8")
        fake_binary_directory = self.root / "bin"
        fake_binary_directory.mkdir()
        fake_gh = fake_binary_directory / "gh"
        fake_gh.write_text(
            "#!/bin/sh\nset -eu\ndestination=''\n"
            "while [ \"$#\" -gt 0 ]; do\n"
            "  if [ \"$1\" = '--dir' ]; then destination=\"$2\"; shift 2; else shift; fi\n"
            "done\n"
            "cp \"$FAKE_RELEASE_DIR/manifest.json\" \"$destination/manifest.json\"\n"
            "cp \"$FAKE_RELEASE_DIR/payloads/release-assets/pages.tar.gz\" \"$destination/pages.tar.gz\"\n",
            encoding="utf-8",
        )
        fake_gh.chmod(0o755)
        fake_curl = fake_binary_directory / "curl"
        fake_curl.write_text("#!/bin/sh\ncat \"$FAKE_PUBLIC_MARKER\"\n", encoding="utf-8")
        fake_curl.chmod(0o755)
        deploy_environment = environment | {
            "PATH": f"{fake_binary_directory}{os.pathsep}{environment['PATH']}",
            "FAKE_RELEASE_DIR": str(artifact_directory),
            "FAKE_PUBLIC_MARKER": str(public_marker),
            "PAGES_VERIFY_ATTEMPTS": "1",
            "PAGES_VERIFY_DELAY_SECONDS": "0",
        }
        deploy_arguments = (
            "--version",
            RELEASE_VERSION,
            "--url",
            "https://pages.example.invalid",
            "--skip-configure",
        )
        deployed = self.command(str(DEPLOY_PAGES), *deploy_arguments, cwd=self.repo, env=deploy_environment)
        self.assertIn(f"at source {source_commit}", deployed.stdout)
        self.assertNotIn(f"at source {release_commit}", deployed.stdout)
        deployed_marker = json.loads(
            self.command(
                "git", "show", "refs/heads/gh-pages:.mprlab-release.json", cwd=self.remote, git_dir=True
            ).stdout
        )
        self.assertEqual(deployed_marker["source_commit"], source_commit)
        self.command("git", "cat-file", "-e", "refs/heads/gh-pages:.nojekyll", cwd=self.remote, git_dir=True)

        invalid_markers = (
            {**marker, "schema_version": 2},
            {**marker, "release_version": "v9.9.9"},
            {**marker, "source_commit": release_commit},
        )
        for invalid_marker in invalid_markers:
            with self.subTest(marker=invalid_marker):
                public_marker.write_text(json.dumps(invalid_marker), encoding="utf-8")
                rejected = self.command(
                    str(DEPLOY_PAGES),
                    *deploy_arguments,
                    cwd=self.repo,
                    env=deploy_environment,
                    check=False,
                )
                self.assertNotEqual(rejected.returncode, 0)
                self.assertIn(f"source {source_commit}", rejected.stderr)


if __name__ == "__main__":
    unittest.main()
