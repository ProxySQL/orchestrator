#!/usr/bin/env python3
"""Run manually on the signing server; never copy the private key into the bundle."""
import argparse
import os
from pathlib import Path
import shutil
import tempfile

import release


def sign(directory, output=None, gpg="gpg"):
    directory = Path(directory).resolve()
    record = release.load_bundle(directory)
    output = Path(output) if output is not None else directory / "signed"
    release.require(not output.exists() and not output.is_symlink(), "Signing output must be a fresh directory")
    release.require(not output.resolve().is_relative_to(directory / "unsigned"),
                    "Signing output cannot be inside the original unsigned inventory")
    executable = shutil.which(str(gpg))
    release.require(executable and not any(c.isspace() for c in executable),
                    "GPG executable must exist and have no whitespace in its path")
    executable = str(Path(executable).resolve())
    names = release.expected_names(record["build"]["tag"]) | {release.BUILD_INFO}
    output.mkdir(parents=True)
    for name in names:
        shutil.copyfile(directory / "unsigned" / name, output / name)
    # Command-line macros work with the server's RPM 4.17 and newer RPM versions;
    # the modern signing-key macro and the legacy GnuPG macro select the same key.
    release.run(["rpmsign", "--define", f"__gpg {executable}",
                 "--define", f"_gpg_name {release.KEY}",
                 "--define", f"_openpgp_sign_id {release.KEY}",
                 "--define", "_gpg_digest_algo sha256", "--addsign",
                 *[output / name for name in sorted(names) if name.endswith(".rpm")]])
    (output / "SHA256SUMS").write_text(release.checksum_text(release.file_hashes(output, names)))
    release.run([executable, "--local-user", release.KEY, "--digest-algo", "SHA256",
                 "--armor", "--detach-sign", "--output", output / "SHA256SUMS.asc", output / "SHA256SUMS"])
    # Export only the public certificate to verify without changing any keyring.
    with tempfile.TemporaryDirectory(prefix="orchestrator-sign-check-") as scratch:
        public_key = Path(scratch) / "public.asc"
        public_key.write_text(release.run([executable, "--armor", "--export", release.KEY]))
        release.verify(directory, output, public_key)
    return output


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    parser.add_argument("--output", type=Path, help="fresh output directory (default: DIRECTORY/signed)")
    parser.add_argument("--gpg", default="gpg", help="existing GPG executable or server signing wrapper")
    args = parser.parse_args()
    try:
        if os.isatty(0):
            os.environ["GPG_TTY"] = os.ttyname(0)
        print(f"Signed and verified: {sign(args.directory, args.output, args.gpg)}")
    except (release.ReleaseError, OSError, ValueError, KeyError, TypeError) as exc:
        parser.exit(1, f"error: {exc}\n")


if __name__ == "__main__":
    main()
