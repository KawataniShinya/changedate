"""Exercise the cron wrapper with simulated mount tables and a fake binary."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "changedate-ubuntu.sh"


class MountCheckTest(unittest.TestCase):
    def run_wrapper(self, tool_mount, photo_mount):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tools = root / "tools"
            photos = root / "photos"
            commands = root / "commands"
            for path in (tools, photos, commands):
                path.mkdir()
            marker = root / "executed"
            binary = tools / "changedate-linux"
            binary.write_text('#!/bin/bash\nprintf executed > "$TEST_MARKER"\n')
            binary.chmod(0o755)
            findmnt = commands / "findmnt"
            findmnt.write_text('''#!/bin/bash
filtered=false
field=
target=
while (( $# )); do
  case "$1" in
    -t) [[ "$2" == cifs ]] && filtered=true; shift 2 ;;
    -o) field=$2; shift 2 ;;
    -T) target=$2; shift 2 ;;
    *) shift ;;
  esac
done
state=$TEST_TOOL_MOUNT
[[ "$target" == "$CHANGEDATE_PHOTO_DIR" ]] && state=$TEST_PHOTO_MOUNT
[[ "$state" == missing ]] && exit 1
if [[ "$field" == FSTYPE ]]; then
  if [[ "$state" == automount ]] && ! $filtered; then
    printf 'autofs\\ncifs\\n'
  else
    printf 'cifs\\n'
  fi
else
  if [[ "$state" == ro ]]; then
    printf 'ro,relatime\\n'
  elif [[ "$state" == automount ]] && ! $filtered; then
    printf 'rw,relatime\\nrw,relatime,vers=3.1.1\\n'
  else
    printf 'rw,relatime,vers=3.1.1\\n'
  fi
fi
''')
            findmnt.chmod(0o755)
            env = dict(os.environ, PATH=f"{commands}:/usr/bin:/bin",
                       CHANGEDATE_TOOL_DIR=str(tools),
                       CHANGEDATE_PHOTO_DIR=str(photos),
                       CHANGEDATE_SMB_SHARE="//nas/share",
                       CHANGEDATE_SMB_ROOT=str(root),
                       CHANGEDATE_SMB_AUTH_FILE=str(root / "credentials"),
                       CHANGEDATE_LOCK_FILE=str(root / "lock"),
                       TEST_MARKER=str(marker),
                       TEST_TOOL_MOUNT=tool_mount,
                       TEST_PHOTO_MOUNT=photo_mount)
            result = subprocess.run(["/bin/bash", str(SCRIPT)], env=env,
                                    capture_output=True, text=True)
            return result, marker.exists()

    def test_writable_cifs_with_and_without_automount(self):
        for state in ("rw", "automount"):
            with self.subTest(state=state):
                result, executed = self.run_wrapper(state, state)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertTrue(executed)

    def test_rejects_read_only_or_missing_mount_on_either_path(self):
        for state in ("ro", "missing"):
            for tool, photo in ((state, "rw"), ("rw", state)):
                with self.subTest(tool=tool, photo=photo):
                    result, executed = self.run_wrapper(tool, photo)
                    self.assertEqual(result.returncode, 1, result.stderr)
                    self.assertFalse(executed)
                    self.assertIn("NAS is not mounted read-write", result.stderr)
