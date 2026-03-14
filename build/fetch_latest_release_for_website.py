#!/usr/bin/env python3
import json
import os
import pathlib
import urllib.error
import urllib.request


def main() -> None:
    repo = os.environ.get("GITHUB_REPOSITORY", "championswimmer/env.sync.local")
    token = os.environ.get("GITHUB_TOKEN", "")
    output_path = pathlib.Path(
        os.environ.get("OUTPUT_PATH", "website-app/public/download/latest-release.json")
    )
    output_path.parent.mkdir(parents=True, exist_ok=True)

    headers = {
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"

    req = urllib.request.Request(
        f"https://api.github.com/repos/{repo}/releases/latest",
        headers=headers,
    )

    try:
        with urllib.request.urlopen(req) as response:
            release = json.load(response)
    except urllib.error.URLError as err:
        print(f"Failed to fetch latest release details: {err}")
        release = {"tag_name": "", "assets": []}

    filtered = {
        "tag_name": release.get("tag_name", ""),
        "assets": [
            {
                "name": asset.get("name", ""),
                "size": asset.get("size", 0),
                "browser_download_url": asset.get("browser_download_url", ""),
            }
            for asset in release.get("assets", [])
        ],
    }

    output_path.write_text(json.dumps(filtered, indent=2) + "\n", encoding="utf-8")
    print(f"Wrote {output_path}")


if __name__ == "__main__":
    main()
