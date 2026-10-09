#!/usr/bin/env python3
"""Format first-party sources; exclude generated catalogs and the upstream submodule."""
import argparse
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    args = parser.parse_args()
    files = subprocess.check_output(
        ['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=ROOT
    ).decode().split('\0')
    files = sorted({name for name in files if name and (ROOT / name).is_file()})
    native = [name for name in files if Path(name).suffix in {'.c', '.h', '.m', '.mm'}
              and not name.endswith('.generated.h')]
    golang = [name for name in files if name.endswith(('.go', '.go.tmpl'))]
    version = subprocess.check_output(['clang-format', '--version'], text=True)
    if 'version 18.' not in version:
        raise SystemExit('Use clang-format 18 (CI uses 18.1.8).')
    subprocess.run(['clang-format', *(['--dry-run', '--Werror'] if args.check else ['-i']),
                    *native], cwd=ROOT, check=True)
    if args.check:
        pending = subprocess.check_output(['gofmt', '-l', *golang], cwd=ROOT, text=True)
        if pending:
            raise SystemExit('Run python3 scripts/format.py:\n' + pending)
    else:
        subprocess.run(['gofmt', '-w', *golang], cwd=ROOT, check=True)


if __name__ == '__main__':
    main()
