"""MkDocs hook: publish the repository's assets/ images in the site.

The README and the docs share assets/ (logo, demo GIFs, screenshots). The hook
adds them to the site as assets/..., so no copy has to be committed under docs/.
"""

from pathlib import Path

from mkdocs.structure.files import File

PUBLISHED = ["pandora-logo.svg", "demo.gif", "demo-cli.gif", "screenshots"]


def on_files(files, config):
    root = Path(config.config_file_path).parent
    for name in PUBLISHED:
        base = root / "assets" / name
        paths = sorted(base.rglob("*")) if base.is_dir() else [base]
        for path in paths:
            if path.is_file():
                rel = path.relative_to(root).as_posix()
                files.append(File(rel, str(root), config.site_dir, config.use_directory_urls))
    return files
