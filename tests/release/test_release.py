import importlib.util
import pathlib
import json
import os
import shutil
import subprocess
from unittest.mock import patch
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]


class ReleaseTestCase(unittest.TestCase):
    def setUp(self):
        path = ROOT / "script/release.py"
        self.assertTrue(path.exists(), "release inventory command is missing")
        spec = importlib.util.spec_from_file_location("release", path)
        self.release = importlib.util.module_from_spec(spec)
        sys.modules["release"] = self.release
        spec.loader.exec_module(self.release)
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = pathlib.Path(self.temp.name)

    def populate(self, tag="v4.32.0", arch=None):
        for name in self.release.expected_names(tag, arch):
            (self.directory / name).write_bytes(b"package")


    def bundle(self, rpm=None, tag="v4.32.0"):
        directory = self.directory / "bundle"
        unsigned = directory / "unsigned"
        unsigned.mkdir(parents=True)
        names = self.release.expected_names(tag)
        for name in names:
            (unsigned / name).write_bytes(pathlib.Path(rpm).read_bytes() if rpm and name.endswith(".rpm") else b"original package")
        info = {"schema": 1, "repository": "ProxySQL/orchestrator", "tag": tag,
                "commit": "a" * 40, "workflow_run": 123, "workflow_attempt": 1,
                "unsigned_sha256": self.release.file_hashes(unsigned, names)}
        self.release.write_json(unsigned / self.release.BUILD_INFO, info)
        hashes = self.release.file_hashes(unsigned, names | {self.release.BUILD_INFO})
        assets = [{"id": i, "name": name, "size": (unsigned / name).stat().st_size,
                   "digest": "sha256:" + hashes[name]} for i, name in enumerate(sorted(hashes), 1)]
        meta = {"id": 100, "tag_name": tag, "draft": True, "prerelease": "rc" in tag,
                "target_commitish": info["commit"], "assets": assets}
        record = {"schema": 1, "repository": info["repository"], "release": meta,
                  "build": info, "unsigned_sha256": hashes}
        self.release.write_json(directory / "release.json", record)
        (directory / "UNSIGNED-SHA256SUMS").write_text(self.release.checksum_text(hashes))
        return directory, record


class InventoryTests(ReleaseTestCase):
    def test_complete_release_has_fourteen_packages(self):
        names = self.release.expected_names("v4.32.0")
        self.assertEqual(len(names), 14)
        self.assertIn("orchestrator-client_4.32.0_arm64.deb", names)
        self.assertNotIn("orchestrator-cli-4.32.0-linux-amd64.tar.gz", names)
        self.populate()
        self.assertEqual(self.release.check_packages(self.directory, "v4.32.0"), names)

    def test_architecture_inventory_has_seven_packages(self):
        for arch in ("amd64", "arm64"):
            self.assertEqual(len(self.release.expected_names("v4.32.0", arch)), 7)

    def test_prerelease_names_match_fpm(self):
        names = self.release.expected_names("v4.30.1-rc2")
        self.assertIn("orchestrator-4.30.1_rc2-1.x86_64.rpm", names)
        self.assertIn("orchestrator_4.30.1-rc2_amd64.deb", names)
        self.assertIn("orchestrator-4.30.1-rc2-linux-amd64.tar.gz", names)

    def test_missing_architecture_is_rejected(self):
        self.populate(arch="amd64")
        with self.assertRaises(self.release.ReleaseError):
            self.release.check_packages(self.directory, "v4.32.0")

    def test_unexpected_file_is_rejected(self):
        self.populate()
        (self.directory / "other.rpm").write_bytes(b"unrelated release")
        with self.assertRaises(self.release.ReleaseError):
            self.release.check_packages(self.directory, "v4.32.0")

    def test_symlink_package_is_rejected(self):
        self.populate()
        target = self.directory / "orchestrator-4.32.0-1.x86_64.rpm"
        target.unlink()
        target.symlink_to("orchestrator-4.32.0-1.aarch64.rpm")
        with self.assertRaises(self.release.ReleaseError):
            self.release.check_packages(self.directory, "v4.32.0")

    def test_empty_package_is_rejected(self):
        self.populate()
        (self.directory / "orchestrator-4.32.0-1.x86_64.rpm").write_bytes(b"")
        with self.assertRaises(self.release.ReleaseError):
            self.release.check_packages(self.directory, "v4.32.0")

    def test_unsafe_tag_is_rejected(self):
        for tag in ("../v4.32.0", "--help", "v4.32.0/evil", "v4.32.0\n"):
            with self.assertRaises(self.release.ReleaseError):
                self.release.expected_names(tag)



class BundleTests(ReleaseTestCase):
    def test_original_checksums_are_required(self):
        directory, record = self.bundle()
        self.assertEqual(self.release.load_bundle(directory), record)
        (directory / "unsigned" / "orchestrator_4.32.0_amd64.deb").write_bytes(b"changed")
        with self.assertRaises(self.release.ReleaseError):
            self.release.load_bundle(directory)

    def test_unsigned_manifest_must_cover_exact_inventory(self):
        directory, _ = self.bundle()
        (directory / "UNSIGNED-SHA256SUMS").write_text("partial inventory")
        with self.assertRaises(self.release.ReleaseError):
            self.release.load_bundle(directory)

    def test_draft_lookup_resolves_cli_id_before_rest_request(self):
        directory, record = self.bundle()
        with patch.object(self.release, "run", return_value='{"databaseId": 100}') as command, patch.object(self.release, "api", return_value=record["release"]) as api:
            meta = self.release.release_metadata("ProxySQL/orchestrator", "v4.32.0")
        self.assertTrue(meta["draft"])
        self.assertEqual(command.call_args.args[0][:4], ["gh", "release", "view", "v4.32.0"])
        api.assert_called_once_with("ProxySQL/orchestrator", "releases/100")

    def test_prepare_downloads_one_complete_draft_with_matching_successful_run(self):
        directory, record = self.bundle()
        target = self.directory / "prepared"
        run = {"id": 123, "run_attempt": 1, "event": "push", "head_branch": "v4.32.0",
               "head_sha": "a" * 40, "path": ".github/workflows/release.yml",
               "status": "completed", "conclusion": "success", "repository": {"full_name": "ProxySQL/orchestrator"}}
        responses = {"releases/tags/v4.32.0": record["release"], "commits/v4.32.0": {"sha": "a" * 40},
                     "actions/runs/123": run}
        def download(repo, tag, dest):
            shutil.copytree(directory / "unsigned", dest)
        with patch.object(self.release, "api", side_effect=lambda repo, path: responses[path]), patch.object(self.release, "release_metadata", return_value=record["release"]), patch.object(self.release, "download", side_effect=download):
            self.release.prepare("v4.32.0", target, "ProxySQL/orchestrator")
        self.assertEqual(self.release.load_bundle(target)["build"], record["build"])

    def test_prepare_rejects_published_release(self):
        directory, record = self.bundle()
        record["release"]["draft"] = False
        with patch.object(self.release, "release_metadata", return_value=record["release"]), patch.object(self.release, "download") as download:
            with self.assertRaises(self.release.ReleaseError):
                self.release.prepare("v4.32.0", self.directory / "prepared", "ProxySQL/orchestrator")
            download.assert_not_called()

    def test_prepare_rejects_reused_directory(self):
        with self.assertRaises(self.release.ReleaseError):
            self.release.prepare("v4.32.0", self.directory, "ProxySQL/orchestrator")

    def test_changed_draft_identity_is_rejected(self):
        directory, record = self.bundle()
        for field, value in (("id", 101), ("draft", False), ("prerelease", True), ("target_commitish", "b" * 40)):
            meta = dict(record["release"], **{field: value})
            with self.subTest(field=field), self.assertRaises(self.release.ReleaseError):
                self.release.require_draft(record, meta)

    def test_upload_cannot_publish_and_refuses_changed_release(self):
        directory, record = self.bundle()
        changed = dict(record["release"], id=101)
        with patch.object(self.release, "verify", return_value={}), patch.object(self.release, "release_metadata", return_value=changed), patch.object(self.release, "run") as run:
            with self.assertRaises(self.release.ReleaseError):
                self.release.upload(directory)
            run.assert_not_called()

    def test_publication_requires_verified_remote_assets(self):
        directory, _ = self.bundle()
        self.release.write_json(directory / "uploaded.json", {"release_id": 100, "sha256": {}})
        with patch.object(self.release, "verify_remote", side_effect=self.release.ReleaseError("invalid signature")) as remote, patch.object(self.release, "run") as run:
            with self.assertRaises(self.release.ReleaseError):
                self.release.publish(directory)
            remote.assert_called_once()
            run.assert_not_called()


CRYPTO_TOOLS = ("gpg", "rpm", "rpmkeys", "rpmbuild", "rpmsign")


@unittest.skipUnless(all(shutil.which(tool) for tool in CRYPTO_TOOLS), "requires GnuPG and RPM build/sign tools")
class CryptoTests(ReleaseTestCase):
    @classmethod
    def setUpClass(cls):
        cls.crypto = tempfile.TemporaryDirectory()
        root = pathlib.Path(cls.crypto.name)
        cls.home = root / "gnupg"
        cls.home.mkdir(mode=0o700)
        gpg = ["gpg", "--homedir", str(cls.home), "--batch", "--pinentry-mode", "loopback", "--passphrase", ""]
        subprocess.run(gpg + ["--quick-gen-key", "Release test <release@example.invalid>", "rsa2048", "sign", "0"], check=True, capture_output=True)
        listing = subprocess.check_output(gpg + ["--with-colons", "--list-keys"], text=True)
        cls.fingerprint = next(line.split(":")[9] for line in listing.splitlines() if line.startswith("fpr:"))
        cls.public_key = root / "key.asc"
        cls.public_key.write_bytes(subprocess.check_output(gpg + ["--armor", "--export", cls.fingerprint]))
        cls.rpms = []
        top = root / "rpm"
        for marker in ("original", "different"):
            spec = root / (marker + ".spec")
            spec.write_text("Name: release-test\nVersion: 1\nRelease: 1\nSummary: Signing integration fixture\nLicense: Apache-2.0\nBuildArch: noarch\n%description\nTemporary test fixture.\n%install\nmkdir -p %{buildroot}/usr/share/release-test\nprintf " + marker + " > %{buildroot}/usr/share/release-test/marker\n%files\n/usr/share/release-test/marker\n")
            subprocess.run(["rpmbuild", "--define", "_topdir " + str(top), "-bb", str(spec)], check=True, capture_output=True)
            rpm = root / (marker + ".rpm")
            shutil.copyfile(next((top / "RPMS").rglob("*.rpm")), rpm)
            cls.rpms.append(rpm)

    @classmethod
    def tearDownClass(cls):
        subprocess.run(["gpgconf", "--homedir", str(cls.home), "--kill", "gpg-agent"], capture_output=True)
        cls.crypto.cleanup()

    def setUp(self):
        super().setUp()
        self.release.KEY = self.fingerprint
        self.env = patch.dict(os.environ, {"GNUPGHOME": str(self.home)})
        self.env.start()
        self.addCleanup(self.env.stop)
        path = ROOT / "script/sign-release.py"
        self.assertTrue(path.exists(), "server-side signing command is missing")
        spec = importlib.util.spec_from_file_location("sign_release", path)
        self.signer = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.signer)

    def signed_bundle(self, tag="v4.32.0"):
        directory, _ = self.bundle(self.rpms[0], tag)
        output = self.signer.sign(directory)
        return directory, pathlib.Path(output)

    def resign_manifest(self, signed):
        names = self.release.expected_names("v4.32.0") | {self.release.BUILD_INFO}
        (signed / "SHA256SUMS").write_text(self.release.checksum_text(self.release.file_hashes(signed, names)))
        self.gpg_manifest(signed)

    def gpg_manifest(self, signed):
        subprocess.run(["gpg", "--batch", "--yes", "--local-user", self.fingerprint, "--digest-algo", "SHA256", "--armor", "--detach-sign", "--output", str(signed / "SHA256SUMS.asc"), str(signed / "SHA256SUMS")], check=True, capture_output=True)

    def test_real_signing_and_isolated_verification(self):
        directory, signed = self.signed_bundle()
        hashes = self.release.verify(directory, signed, self.public_key)
        self.assertEqual(len(hashes), 17)
        self.assertNotEqual(self.release.sha256(signed / "orchestrator-4.32.0-1.x86_64.rpm"), self.release.sha256(directory / "unsigned/orchestrator-4.32.0-1.x86_64.rpm"))

    def test_changed_deb_with_valid_manifest_signature_is_rejected(self):
        directory, signed = self.signed_bundle()
        (signed / "orchestrator_4.32.0_amd64.deb").write_bytes(b"changed by signer")
        self.resign_manifest(signed)
        with self.assertRaises(self.release.ReleaseError):
            self.release.verify(directory, signed, self.public_key)

    def test_unsigned_rpm_with_valid_manifest_is_rejected(self):
        directory, signed = self.signed_bundle()
        name = "orchestrator-4.32.0-1.x86_64.rpm"
        shutil.copyfile(directory / "unsigned" / name, signed / name)
        self.resign_manifest(signed)
        with self.assertRaisesRegex(self.release.ReleaseError, "signature"):
            self.release.verify(directory, signed, self.public_key)

    def test_changed_rpm_payload_with_valid_signatures_is_rejected(self):
        directory, signed = self.signed_bundle()
        target = signed / "orchestrator-4.32.0-1.x86_64.rpm"
        shutil.copyfile(self.rpms[1], target)
        subprocess.run(["rpmsign", "--define", "_gpg_name " + self.fingerprint, "--define", "_gpg_digest_algo sha256", "--addsign", str(target)], check=True, capture_output=True)
        self.resign_manifest(signed)
        with self.assertRaisesRegex(self.release.ReleaseError, "header|payload"):
            self.release.verify(directory, signed, self.public_key)

    def test_incomplete_signed_manifest_is_rejected(self):
        directory, signed = self.signed_bundle()
        sums = signed / "SHA256SUMS"
        sums.write_text("".join(sums.read_text().splitlines(keepends=True)[1:]))
        self.gpg_manifest(signed)
        with self.assertRaises(self.release.ReleaseError):
            self.release.verify(directory, signed, self.public_key)

    def test_unapproved_full_fingerprint_is_rejected(self):
        directory, signed = self.signed_bundle()
        self.release.KEY = "0" * 40
        with self.assertRaisesRegex(self.release.ReleaseError, "fingerprint"):
            self.release.verify(directory, signed, self.public_key)


    def remote_api(self, record, state):
        def api(repo, path):
            self.assertEqual(repo, record["repository"])
            if path.startswith("releases/tags/"):
                return json.loads(json.dumps(state))
            if path.startswith("commits/"):
                return {"sha": record["build"]["commit"]}
            self.assertEqual(path, "actions/runs/123")
            return {"id": 123, "run_attempt": 1, "event": "push", "head_branch": record["build"]["tag"],
                    "head_sha": record["build"]["commit"], "path": ".github/workflows/release.yml",
                    "status": "completed", "conclusion": "success", "repository": {"full_name": repo}}
        return api

    def remote_assets(self, record, signed):
        old = {a["name"]: a for a in record["release"]["assets"]}
        return [{"id": old[p.name]["id"] if p.name in old else 1000 + i,
                 "name": p.name, "size": p.stat().st_size,
                 "digest": "sha256:" + self.release.sha256(p), "download_count": 0}
                for i, p in enumerate(sorted(signed.iterdir()))]

    def test_remote_verification_allows_download_counter_updates(self):
        directory, signed = self.signed_bundle()
        record = self.release.load_bundle(directory)
        state = dict(record["release"], assets=self.remote_assets(record, signed))
        def download(repo, tag, dest):
            shutil.copytree(signed, dest)
            for asset in state["assets"]:
                asset["download_count"] += 1
        with patch.object(self.release, "api", side_effect=self.remote_api(record, state)), patch.object(self.release, "release_metadata", side_effect=lambda repo, tag: json.loads(json.dumps(state))), patch.object(self.release, "download", side_effect=download):
            hashes = self.release.verify_remote(directory, self.public_key)
        self.assertEqual(len(hashes), 17)

    def test_partial_upload_resumes_without_publishing_then_explicit_publish_preserves_rc(self):
        directory, signed = self.signed_bundle("v4.32.0-rc1")
        record = self.release.load_bundle(directory)
        complete = self.remote_assets(record, signed)
        state = json.loads(json.dumps(record["release"]))
        missing = "orchestrator-4.32.0_rc1-1.x86_64.rpm"
        state["assets"] = [a for a in state["assets"] if a["name"] != missing]
        commands = []
        original_run = self.release.run
        def run(args, **kwargs):
            if args[0] != "gh":
                return original_run(args, **kwargs)
            commands.append([str(a) for a in args])
            if args[2] == "upload":
                names = {pathlib.Path(p).name for p in args[7:]}
                by_name = {a["name"]: a for a in state["assets"]}
                by_name.update({a["name"]: a for a in complete if a["name"] in names})
                state["assets"] = list(by_name.values())
            else:
                self.assertEqual(args[2], "edit")
                self.assertIn("--draft=false", args)
                state["draft"] = False
            return ""
        def download(repo, tag, dest):
            shutil.copytree(signed, dest)
        with patch.object(self.release, "api", side_effect=self.remote_api(record, state)), patch.object(self.release, "release_metadata", side_effect=lambda repo, tag: json.loads(json.dumps(state))), patch.object(self.release, "download", side_effect=download), patch.object(self.release, "run", side_effect=run):
            hashes = self.release.upload(directory, signed, self.public_key)
            self.assertTrue(state["draft"])
            self.assertEqual(len(hashes), 17)
            self.assertTrue(all(command[2] == "upload" for command in commands))
            self.release.publish(directory, self.public_key)
        self.assertFalse(state["draft"])
        self.assertTrue(state["prerelease"])
        self.assertEqual(sum(command[2] == "edit" for command in commands), 1)
        self.assertTrue((directory / "uploaded.json").is_file())

    def test_manifest_signed_by_another_key_is_rejected(self):
        directory, signed = self.signed_bundle()
        subprocess.run(["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase", "", "--quick-gen-key", "Wrong signing key <wrong@example.invalid>", "rsa2048", "sign", "0"], check=True, capture_output=True)
        listing = subprocess.check_output(["gpg", "--batch", "--with-colons", "--list-keys", "wrong@example.invalid"], text=True)
        wrong = next(line.split(":")[9] for line in listing.splitlines() if line.startswith("fpr:"))
        subprocess.run(["gpg", "--batch", "--yes", "--local-user", wrong, "--digest-algo", "SHA256", "--armor", "--detach-sign", "--output", str(signed / "SHA256SUMS.asc"), str(signed / "SHA256SUMS")], check=True, capture_output=True)
        with self.assertRaises(self.release.ReleaseError):
            self.release.verify(directory, signed, self.public_key)

    def test_signing_refuses_changed_original_inventory(self):
        directory, _ = self.bundle(self.rpms[0])
        (directory / "unsigned/orchestrator_4.32.0_amd64.deb").write_bytes(b"changed original")
        with self.assertRaises(self.release.ReleaseError):
            self.signer.sign(directory)
        self.assertFalse((directory / "signed").exists())


    def test_signing_output_cannot_be_inside_original_inventory(self):
        directory, record = self.bundle(self.rpms[0])
        output = directory / "unsigned" / "nested-output"
        with self.assertRaises(self.release.ReleaseError):
            self.signer.sign(directory, output=output)
        self.assertFalse(output.exists())
        self.assertEqual(self.release.load_bundle(directory), record)


if __name__ == "__main__":
    unittest.main()
