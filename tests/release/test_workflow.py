"""Execute the actual draft-creation shell step against a fake GitHub CLI."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class DraftCreationTests(unittest.TestCase):
    def execute(self, tags="", fail_api=False, prerelease=False):
        text = (ROOT / ".github/workflows/release.yml").read_text()
        lines = text[text.index("    - name: Create draft for manual signing\n"):].splitlines()
        start = lines.index("      run: |") + 1
        body = []
        for line in lines[start:]:
            if line and not line.startswith("        "):
                break
            body.append(line[8:] if line else "")
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch)
            command = root / "gh"
            command.write_text('''#!/usr/bin/env python3
import json, os, sys
with open(os.environ["TEST_GH_LOG"], "a") as out:
    out.write(json.dumps(sys.argv[1:]) + "\\n")
if sys.argv[1] == "api":
    if os.environ["TEST_GH_FAIL"] == "true":
        sys.exit(7)
    print(os.environ["TEST_GH_TAGS"])
''')
            command.chmod(0o755)
            log = root / "commands.jsonl"
            env = dict(os.environ, PATH=str(root) + os.pathsep + os.environ["PATH"],
                       TAG_NAME="v4.32.0", GITHUB_REPOSITORY="ProxySQL/orchestrator", GITHUB_SHA="a" * 40,
                       PRERELEASE=str(prerelease).lower(), TEST_GH_LOG=str(log), TEST_GH_TAGS=tags,
                       TEST_GH_FAIL=str(fail_api).lower())
            result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", "\n".join(body)],
                                    cwd=root, env=env, capture_output=True, text=True)
            commands = [json.loads(line) for line in log.read_text().splitlines()]
            return result, commands

    def test_existing_release_prevents_draft_creation(self):
        result, commands = self.execute(tags="v4.31.1\nv4.32.0")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(command[:2] == ["release", "create"] for command in commands))

    def test_api_failure_cannot_bypass_existing_release_check(self):
        result, commands = self.execute(fail_api=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(command[:2] == ["release", "create"] for command in commands))

    def test_new_tag_creates_one_draft_and_preserves_prerelease(self):
        result, commands = self.execute(tags="v4.31.1", prerelease=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        create = [command for command in commands if command[:2] == ["release", "create"]]
        self.assertEqual(len(create), 1)
        self.assertIn("--draft", create[0])
        self.assertIn("--prerelease", create[0])


if __name__ == "__main__":
    unittest.main()
