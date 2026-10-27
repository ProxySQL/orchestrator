#!/usr/bin/env python3
"""Prepare and verify manually signed package releases. Requires Python 3.9+."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
from urllib.parse import quote
from urllib.request import urlopen

KEY = "653F85BB38256DF8A96206C3E8CA2E8D8217C97E"
KEY_URL = "https://repo.proxysql.com/ProxySQL/proxysql-3.0.x/repo_pub_key"
BUILD_INFO = "BUILD-INFO.json"
REPOSITORY = "ProxySQL/orchestrator"
VARIANTS = ("orchestrator", "orchestrator-cli", "orchestrator-client")
ARCHES = {"amd64": "x86_64", "arm64": "aarch64"}


class ReleaseError(Exception):
    """A release failed a required integrity or state check."""


def require(condition, message):
    if not condition:
        raise ReleaseError(message)


def run(args, **kwargs):
    result = subprocess.run([str(a) for a in args], capture_output=True, text=True, **kwargs)
    if result.returncode:
        raise ReleaseError(f"Command failed: {args[0]}\n{result.stdout}{result.stderr}")
    return result.stdout


def api(repo, path):
    return json.loads(run(["gh", "api", f"repos/{repo}/{path}"]))


def release_metadata(repo, tag):
    # The REST by-tag endpoint only finds published releases. gh resolves drafts
    # by their pending tag as well; fetch the full REST record by numeric ID.
    reference = json.loads(run(["gh", "release", "view", tag, "--repo", repo, "--json", "databaseId"]))
    release_id = reference.get("databaseId")
    require(type(release_id) is int and release_id > 0, "Could not resolve release identity")
    return api(repo, f"releases/{release_id}")


def validate_repo(repo):
    require(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*", repo),
            "Expected repository in OWNER/REPO form")


def expected_names(tag, arch=None):
    require(re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9][A-Za-z0-9_.+-]*)?", tag),
            "Expected a version tag such as v4.32.0 or v4.32.1-rc1")
    require(arch is None or arch in ARCHES, "Unsupported architecture")
    version = tag[1:]
    names = set()
    for goarch in ([arch] if arch else ARCHES):
        names.add(f"orchestrator-{version}-linux-{goarch}.tar.gz")
        for variant in VARIANTS:
            names.add(f"{variant}_{version}_{goarch}.deb")
            names.add(f"{variant}-{version.replace('-', '_')}-1.{ARCHES[goarch]}.rpm")
    return names


def strict_files(directory, names):
    directory = Path(directory)
    require(directory.is_dir() and not directory.is_symlink(), f"Not a plain directory: {directory}")
    actual = {p.name for p in directory.iterdir()}
    require(actual == set(names), f"Inventory mismatch in {directory}: missing={sorted(set(names)-actual)}, extra={sorted(actual-set(names))}")
    for name in names:
        p = directory / name
        require(p.is_file() and not p.is_symlink() and p.stat().st_size > 0, f"Not a nonempty regular file: {p}")


def check_packages(directory, tag, arch=None):
    names = expected_names(tag, arch)
    strict_files(directory, names)
    return names


def sha256(path):
    digest = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def file_hashes(directory, names):
    return {name: sha256(Path(directory) / name) for name in sorted(names)}


def checksum_text(hashes):
    return "".join(f"{hashes[name]}  {name}\n" for name in sorted(hashes))


def write_json(path, value):
    Path(path).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")


def build_info(directory, tag):
    names = check_packages(directory, tag)
    repo = os.environ["GITHUB_REPOSITORY"]
    validate_repo(repo)
    commit = os.environ["GITHUB_SHA"]
    require(re.fullmatch(r"[0-9a-f]{40}", commit), "Invalid workflow commit")
    info = {"schema": 1, "repository": repo, "tag": tag, "commit": commit,
            "workflow_run": int(os.environ["GITHUB_RUN_ID"]),
            "workflow_attempt": int(os.environ["GITHUB_RUN_ATTEMPT"]),
            "unsigned_sha256": file_hashes(directory, names)}
    write_json(Path(directory) / BUILD_INFO, info)



def validate_info(info, repo, tag):
    require(info.get("schema") == 1 and info.get("repository") == repo and info.get("tag") == tag,
            "Build provenance does not match the repository and tag")
    require(re.fullmatch(r"[0-9a-f]{40}", info.get("commit", "")), "Invalid build commit")
    for field in ("workflow_run", "workflow_attempt"):
        require(type(info.get(field)) is int and info[field] > 0, f"Invalid {field}")
    hashes = info.get("unsigned_sha256", {})
    require(set(hashes) == expected_names(tag), "Build provenance has an incomplete package inventory")
    require(all(re.fullmatch(r"[0-9a-f]{64}", value) for value in hashes.values()), "Invalid build checksums")


def assets_by_name(meta, names):
    assets = meta.get("assets", [])
    require(len(assets) == len(names) and {a["name"] for a in assets} == set(names),
            "GitHub release asset inventory is incomplete, duplicated or unexpected")
    for asset in assets:
        require(re.fullmatch(r"sha256:[0-9a-f]{64}", asset.get("digest") or ""),
                f"GitHub has not recorded a SHA256 digest for {asset['name']}")
        require(type(asset.get("size")) is int and asset["size"] > 0, "Invalid GitHub asset size")
    return {a["name"]: a for a in assets}


def download(repo, tag, directory):
    require(not Path(directory).exists(), f"Download directory already exists: {directory}")
    run(["gh", "release", "download", tag, "--repo", repo, "--dir", directory])


def require_draft(record, meta):
    before = record["release"]
    require(meta.get("draft") is True, "Release must still be a draft")
    require(not meta.get("immutable", False), "Release is immutable")
    for field in ("id", "tag_name", "prerelease", "target_commitish"):
        require(meta.get(field) == before.get(field), f"Draft {field} changed since preparation")


def check_source(record):
    repo, info = record["repository"], record["build"]
    tag = info["tag"]
    commit = api(repo, f"commits/{quote(tag, safe='')}")["sha"]
    require(commit == info["commit"], "Release tag no longer points at the prepared commit")
    workflow = api(repo, f"actions/runs/{info['workflow_run']}/attempts/{info['workflow_attempt']}")
    expected = {"id": info["workflow_run"], "run_attempt": info["workflow_attempt"],
                "event": "push", "head_branch": tag, "head_sha": commit,
                "path": ".github/workflows/release.yml", "status": "completed", "conclusion": "success"}
    require(all(workflow.get(k) == v for k, v in expected.items())
            and workflow.get("repository", {}).get("full_name") == repo,
            "Build workflow run/attempt does not match a successful release build")


def prepare(tag, directory, repo=REPOSITORY):
    directory = Path(directory)
    validate_repo(repo)
    names = expected_names(tag) | {BUILD_INFO}
    require(not directory.exists() and not directory.is_symlink(), "Preparation requires a fresh directory")
    meta = release_metadata(repo, tag)
    require(meta.get("draft") is True and meta.get("tag_name") == tag, "Expected the requested draft release")
    assets = assets_by_name(meta, names)
    directory.mkdir(parents=True)
    download(repo, tag, directory / "unsigned")
    strict_files(directory / "unsigned", names)
    hashes = file_hashes(directory / "unsigned", names)
    for name, asset in assets.items():
        require(asset["digest"] == "sha256:" + hashes[name]
                and asset["size"] == (directory / "unsigned" / name).stat().st_size,
                f"Downloaded asset differs from GitHub's checksum: {name}")
    info = json.loads((directory / "unsigned" / BUILD_INFO).read_text())
    validate_info(info, repo, tag)
    require(info["unsigned_sha256"] == {name: hashes[name] for name in expected_names(tag)},
            "Downloaded packages differ from CI build provenance")
    require(meta.get("target_commitish") == info["commit"], "Draft target differs from the build commit")
    record = {"schema": 1, "repository": repo, "release": meta, "build": info, "unsigned_sha256": hashes}
    require_draft(record, meta)
    check_source(record)
    write_json(directory / "release.json", record)
    (directory / "UNSIGNED-SHA256SUMS").write_text(checksum_text(hashes))
    load_bundle(directory)
    return directory


def load_bundle(directory):
    directory = Path(directory)
    require(directory.is_dir() and not directory.is_symlink(), "Expected a plain prepared directory")
    for name in ("release.json", "UNSIGNED-SHA256SUMS"):
        path = directory / name
        require(path.is_file() and not path.is_symlink(), f"Missing trusted local record: {name}")
    record = json.loads((directory / "release.json").read_text())
    require(record.get("schema") == 1, "Unknown preparation record schema")
    repo, info = record["repository"], record["build"]
    validate_repo(repo)
    tag = info["tag"]
    validate_info(info, repo, tag)
    names = expected_names(tag) | {BUILD_INFO}
    strict_files(directory / "unsigned", names)
    hashes = file_hashes(directory / "unsigned", names)
    require(hashes == record["unsigned_sha256"], "Unsigned package/provenance checksums changed")
    require((directory / "UNSIGNED-SHA256SUMS").read_text() == checksum_text(hashes),
            "Unsigned checksum manifest is incomplete or changed")
    require(json.loads((directory / "unsigned" / BUILD_INFO).read_text()) == info,
            "Build provenance changed")
    require(info["unsigned_sha256"] == {name: hashes[name] for name in expected_names(tag)},
            "Build provenance package checksums differ")
    assets = assets_by_name(record["release"], names)
    require(record["release"]["tag_name"] == tag and record["release"]["target_commitish"] == info["commit"],
            "Prepared release identity differs from build provenance")
    for name, asset in assets.items():
        require(asset["digest"] == "sha256:" + hashes[name]
                and asset["size"] == (directory / "unsigned" / name).stat().st_size,
                f"Original asset record changed: {name}")
    return record


def verification_keys(temp, public_key):
    home = temp / "gnupg"
    home.mkdir(mode=0o700)
    keyfile = temp / "key.asc"
    if public_key is None:
        with urlopen(KEY_URL, timeout=30) as response:
            keyfile.write_bytes(response.read())
    else:
        keyfile.write_bytes(Path(public_key).read_bytes())
    gpg = ["gpg", "--no-options", "--homedir", home, "--batch", "--no-auto-key-retrieve", "--auto-key-locate", "clear"]
    listing = run(gpg + ["--with-colons", "--import-options", "show-only", "--import", keyfile])
    rows = [line.split(":") for line in listing.splitlines()]
    primary = [i for i, row in enumerate(rows) if row[0] == "pub"]
    require(len(primary) == 1 and primary[0] + 1 < len(rows)
            and rows[primary[0] + 1][0] == "fpr" and rows[primary[0] + 1][9] == KEY
            and not any(row[0] in ("sec", "ssb") for row in rows),
            "Public key full fingerprint does not match the single approved key")
    run(gpg + ["--import", keyfile])
    rpm = ["rpm", "--define", "_keyring rpmdb", "--dbpath", temp / "rpmdb"]
    run(rpm + ["--initdb"])
    run(rpm + ["--import", keyfile])
    version = re.search(r"\b([0-9]+)\.", run(["rpm", "--version"]))
    if version and int(version[1]) >= 6:
        keys = run(["rpmkeys", "--define", "_keyring rpmdb", "--dbpath", temp / "rpmdb", "--list"]).splitlines()
        require(len(keys) == 1 and keys[0].lower().startswith(KEY.lower() + " "), "Unexpected RPM keyring contents")
    else:
        keys = run(rpm + ["-qa", "--qf", "%{NAME}-%{VERSION}\n"]).splitlines()
        require(keys == ["gpg-pubkey-" + KEY[-8:].lower()], "Unexpected RPM keyring contents")
    return gpg, rpm


def verify(directory, signed=None, public_key=None):
    directory = Path(directory)
    record = load_bundle(directory)
    signed = Path(signed) if signed is not None else directory / "signed"
    names = expected_names(record["build"]["tag"]) | {BUILD_INFO}
    strict_files(signed, names | {"SHA256SUMS", "SHA256SUMS.asc"})
    hashes = file_hashes(signed, names)
    require((signed / "SHA256SUMS").read_text() == checksum_text(hashes),
            "Signed checksum manifest must cover every package and BUILD-INFO.json exactly once")
    with tempfile.TemporaryDirectory(prefix="orchestrator-verify-") as scratch:
        gpg, rpm = verification_keys(Path(scratch), public_key)
        status = run(gpg + ["--status-fd", "1", "--verify", signed / "SHA256SUMS.asc", signed / "SHA256SUMS"])
        valid = [line.split() for line in status.splitlines() if line.startswith("[GNUPG:] VALIDSIG ")]
        require(len(valid) == 1 and valid[0][2] == KEY and valid[0][9] == "8"
                and not re.search(r"BADSIG|ERRSIG|REVKEYSIG|EXPKEYSIG|KEYEXPIRED|SIGEXPIRED", status),
                "Manifest signature is not a valid SHA256 signature from the approved key")
        for name in sorted(names):
            path = signed / name
            if not name.endswith(".rpm"):
                require(hashes[name] == record["unsigned_sha256"][name], f"Non-RPM file changed during signing: {name}")
                continue
            result = run(["rpmkeys", "--define", "_keyring rpmdb", "--dbpath", Path(scratch) / "rpmdb", "--checksig", "--verbose", path])
            require(re.search(r"(?im)^.*signature.*:\s+OK\s*$", result), f"RPM has no verified signature: {name}")
            signature = run(rpm + ["-qp", "--qf", "%{RSAHEADER:pgpsig}", path])
            require("RSA/SHA256" in signature and KEY[-16:].lower() in signature.lower(), f"RPM signature uses an unexpected key or digest: {name}")
            query = ["-qp", "--qf", "%{SHA256HEADER}\n%{PAYLOADDIGEST}\n"]
            original = run(rpm + query + [directory / "unsigned" / name])
            current = run(rpm + query + [path])
            require(original == current and len(original.splitlines()) == 2
                    and all(re.fullmatch(r"[0-9a-f]{64}", value) for value in current.splitlines()),
                    f"RPM header or payload changed during signing: {name}")
            require(hashes[name] != record["unsigned_sha256"][name], f"RPM was not changed by signing: {name}")
    return file_hashes(signed, names | {"SHA256SUMS", "SHA256SUMS.asc"})


def verify_remote(directory, public_key=None):
    record = load_bundle(directory)
    repo, tag = record["repository"], record["build"]["tag"]
    meta = release_metadata(repo, tag)
    require_draft(record, meta)
    check_source(record)
    names = expected_names(tag) | {BUILD_INFO, "SHA256SUMS", "SHA256SUMS.asc"}
    assets = assets_by_name(meta, names)
    with tempfile.TemporaryDirectory(prefix="orchestrator-upload-check-") as scratch:
        downloaded = Path(scratch) / "download"
        download(repo, tag, downloaded)
        hashes = verify(directory, downloaded, public_key)
        for name, asset in assets.items():
            require(asset["digest"] == "sha256:" + hashes[name]
                    and asset["size"] == (downloaded / name).stat().st_size,
                    f"Uploaded bytes do not match GitHub metadata: {name}")
    after = release_metadata(repo, tag)
    require_draft(record, after)
    after_assets = assets_by_name(after, names)
    require(all(after_assets[name][field] == asset[field]
                for name, asset in assets.items() for field in ("id", "size", "digest")),
            "Release assets changed during download verification")
    return hashes


def upload(directory, signed=None, public_key=None):
    directory = Path(directory)
    record = load_bundle(directory)
    signed = Path(signed) if signed is not None else directory / "signed"
    hashes = verify(directory, signed, public_key)
    repo, tag = record["repository"], record["build"]["tag"]
    meta = release_metadata(repo, tag)
    require_draft(record, meta)
    check_source(record)
    original = assets_by_name(record["release"], record["unsigned_sha256"])
    current_names = {a["name"] for a in meta["assets"]}
    require({name for name in original if not name.endswith(".rpm")} <= current_names <= set(hashes),
            "Draft asset coverage changed since preparation")
    # A failed GitHub upload may leave an empty `starter` asset. Only our
    # replaceable RPM/checksum names may be treated as pending replacements.
    require(len(current_names) == len(meta["assets"]), "Duplicate draft asset names")
    replaceable = {name for name in hashes if name.endswith(".rpm") or name in ("SHA256SUMS", "SHA256SUMS.asc")}
    starters = {a["name"] for a in meta["assets"] if a["name"] in replaceable
                and a.get("state") == "starter" and a.get("size") == 0 and a.get("digest") is None}
    completed = dict(meta, assets=[a for a in meta["assets"] if a["name"] not in starters])
    current = assets_by_name(completed, current_names - starters)
    for name, asset in current.items():
        allowed = {"sha256:" + hashes[name]}
        if name in original:
            allowed.add(original[name]["digest"])
        require(asset["digest"] in allowed, f"Draft asset changed unexpectedly: {name}")
        if name in original and not name.endswith(".rpm"):
            require(asset["id"] == original[name]["id"], f"Unchanged draft asset was replaced: {name}")
    # Replace the RPM files themselves first. The signed manifest is uploaded last.
    pending = [name for name in sorted(hashes) if name not in current or current[name]["digest"] != "sha256:" + hashes[name]]
    for group in ([name for name in pending if name.endswith(".rpm")],
                  [name for name in pending if name in ("SHA256SUMS", "SHA256SUMS.asc")]):
        if group:
            run(["gh", "release", "upload", tag, "--repo", repo, "--clobber", *[signed / name for name in group]])
    require(verify_remote(directory, public_key) == hashes, "Re-downloaded assets differ from verified signing output")
    write_json(directory / "uploaded.json", {"release_id": meta["id"], "sha256": hashes})
    return hashes


def publish(directory, public_key=None):
    directory = Path(directory)
    record = load_bundle(directory)
    receipt = json.loads((directory / "uploaded.json").read_text())
    require(receipt.get("release_id") == record["release"]["id"], "Upload receipt is for another draft")
    require(verify_remote(directory, public_key) == receipt["sha256"], "Uploaded assets changed after verification")
    repo, tag = record["repository"], record["build"]["tag"]
    run(["gh", "release", "edit", tag, "--repo", repo, "--draft=false"])
    meta = release_metadata(repo, tag)
    require(meta.get("draft") is False and meta["id"] == record["release"]["id"]
            and meta["prerelease"] == record["release"]["prerelease"], "Unexpected release state after publication")
    return meta.get("html_url", tag)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    check = commands.add_parser("check-packages", help="require the exact package inventory")
    check.add_argument("tag")
    check.add_argument("directory", type=Path)
    check.add_argument("--arch", choices=ARCHES)
    info = commands.add_parser("build-info", help="record CI provenance from GITHUB_* variables")
    info.add_argument("tag")
    info.add_argument("directory", type=Path)
    prep = commands.add_parser("prepare", help="download one complete draft into a fresh directory")
    prep.add_argument("tag")
    prep.add_argument("directory", type=Path)
    prep.add_argument("--repo", default=REPOSITORY)
    for command in ("verify", "upload", "publish"):
        sub = commands.add_parser(command)
        sub.add_argument("directory", type=Path, help="original trusted preparation directory")
        sub.add_argument("--public-key", type=Path, help="public key file; full fingerprint is always checked")
        if command != "publish":
            sub.add_argument("--signed", type=Path, help="returned signed directory (default: DIRECTORY/signed)")
    args = parser.parse_args()
    try:
        if args.command == "check-packages":
            print(f"Validated {len(check_packages(args.directory, args.tag, args.arch))} packages")
        elif args.command == "build-info":
            build_info(args.directory, args.tag)
        elif args.command == "prepare":
            print(f"Prepared {prepare(args.tag, args.directory, args.repo)}")
        elif args.command in ("verify", "upload"):
            hashes = globals()[args.command](args.directory, args.signed, args.public_key)
            print(f"{args.command}: verified {len(hashes)} files with key {KEY}; release remains a draft")
        else:
            print(f"Published {publish(args.directory, args.public_key)}")
    except (ReleaseError, OSError, ValueError, KeyError, TypeError) as exc:
        parser.exit(1, f"error: {exc}\n")


if __name__ == "__main__":
    main()
