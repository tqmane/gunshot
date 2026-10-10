#!/usr/bin/env python3
"""Project pinned upstream plus the iOS overlay into .build/upstream."""
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[1]
UPSTREAM = ROOT / "GotohpCore/upstream"
OVERLAY = ROOT / "GotohpCore/overlay"
OUTPUT = ROOT / ".build/upstream"


def replace_once(source, old, new):
    """Fail at upstream drift instead of silently dropping an adaptation."""
    if source.count(old) != 1:
        raise ValueError(f"review upstream changes: expected one occurrence of {old!r}")
    return source.replace(old, new, 1)


def prepare_config():
    source = (UPSTREAM / "backend/configmanager.go").read_text()
    preferences = source[
        source.index("type Preferences struct {") : source.index(
            "// Config is the on-disk"
        )
    ]
    setters = source[
        source.index("func (g *ConfigManager) SetProxy") : source.index(
            "func (g *ConfigManager) AddCredentials"
        )
    ]
    template = (OVERLAY / "backend/configmanager.go.tmpl").read_text()
    template = replace_once(template, "// UPSTREAM_PREFERENCES", preferences)
    template = replace_once(template, "// UPSTREAM_SETTERS", setters)
    (OUTPUT / "backend/configmanager.go").write_text(template)
    (OUTPUT / "backend/config_migration_test.go").unlink()


def adapt_transport():
    path = OUTPUT / "backend/httpclient.go"
    source = replace_once(
        path.read_text(), '"compress/gzip"', '"compress/gzip"\n "crypto/tls"'
    )
    # Upstream dereferences a nil TLSClientConfig on an ordinary Go transport.
    verify_tls = "transport.TLSClientConfig.InsecureSkipVerify = false"
    source = replace_once(
        source,
        verify_tls,
        "if transport.TLSClientConfig == nil { transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12} }\n\t"
        + verify_tls,
    )
    source = replace_once(
        source, "transport.TLSClientConfig.InsecureSkipVerify = true", verify_tls
    )
    source = replace_once(source, "Timeout:   0,", "Timeout:   6 * time.Hour,")
    path.write_text(source)


def adapt_upload():
    # A failed remote hash lookup must not cause a duplicate after restart.
    path = OUTPUT / "backend/upload.go"
    source = path.read_text()
    start = source.index("\t\tif err != nil {\n\t\t\t// Non-fatal:")
    end = source.index("\n\t\tif len(mediakey)", start)
    path.write_text(
        source[:start]
        + '\t\tif err != nil { return "", fmt.Errorf("remote duplicate check failed: %w", err) }'
        + source[end:]
    )

    # Session tokens stay in the host; refresh uses its SSO authorizer.
    path = OUTPUT / "backend/api.go"
    entry = "func (a *Api) BearerToken() (string, error) {"
    path.write_text(
        replace_once(
            path.read_text(),
            entry,
            entry
            + "\n if token, native, err := gunshotNativeBearer(a.authData); native { return token, err }",
        )
    )


def adapt_persistence_tests():
    # Keep upstream assertions, translating desktop YAML to the iOS JSON store.
    path = OUTPUT / "backend/googleauth_test.go"
    source = replace_once(path.read_text(), '"proxy: %s"', '`"proxy":"%s"`')
    source = replace_once(source, '"upload_threads: %d"', '`"uploadThreads":%d`')
    path.write_text(source)


def main():
    revision = (ROOT / "GotohpCore/UPSTREAM_REVISION").read_text().strip()
    actual = subprocess.check_output(
        ["git", "-C", str(UPSTREAM), "rev-parse", "HEAD"], text=True
    ).strip()
    if actual != revision:
        raise SystemExit("upstream revision mismatch; initialize the pinned submodule")
    if OUTPUT.exists():
        shutil.rmtree(OUTPUT)
    OUTPUT.mkdir(parents=True)
    for name in ["backend", "generated"]:
        shutil.copytree(UPSTREAM / name, OUTPUT / name)
    for name in ["LICENSE", "go.sum"]:
        shutil.copy2(UPSTREAM / name, OUTPUT / name)
    for path in (OUTPUT / "backend").glob("wails*.go"):
        path.unlink()
    shutil.copy2(OVERLAY / "go.mod", OUTPUT / "go.mod")
    for path in (OVERLAY / "backend").glob("*.go"):
        shutil.copy2(path, OUTPUT / "backend" / path.name)
    prepare_config()
    adapt_transport()
    adapt_upload()
    adapt_persistence_tests()


if __name__ == "__main__":
    main()
