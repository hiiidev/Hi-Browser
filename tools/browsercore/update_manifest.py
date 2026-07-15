#!/usr/bin/env python3
"""Generate the static browser-core manifest from upstream GitHub Releases."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

API_ROOT = "https://api.github.com"
DEFAULT_REPOSITORY = "adryfish/fingerprint-chromium"
SUPPORTED_SUFFIXES = (".zip", ".tar.gz", ".tgz", ".tar.xz", ".txz", ".dmg")
SHA256_RE = re.compile(r"^[a-fA-F0-9]{64}$")


def request_bytes(url: str, token: str = "", accept: str = "application/vnd.github+json") -> bytes:
    parsed = urllib.parse.urlparse(url)
    if parsed.scheme != "https":
        raise RuntimeError(f"refusing non-HTTPS URL: {url}")
    headers = {"Accept": accept, "User-Agent": "Hi-Browser/browser-core-manifest"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    request = urllib.request.Request(url, headers=headers)
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            final_url = urllib.parse.urlparse(response.geturl())
            if final_url.scheme != "https":
                raise RuntimeError(f"redirected to non-HTTPS URL: {response.geturl()}")
            return response.read(8 * 1024 * 1024 + 1)
    except urllib.error.HTTPError as error:
        detail = error.read(1024).decode("utf-8", "replace")
        raise RuntimeError(f"HTTP {error.code} while reading {url}: {detail}") from error


def request_json(url: str, token: str) -> object:
    return json.loads(request_bytes(url, token).decode("utf-8"))


def is_checksum_asset(name: str) -> bool:
    lower = name.lower()
    return (
        "checksum" in lower
        or "sha256sum" in lower
        or lower.endswith(".sha256")
        or lower.endswith(".sha256sum")
    )


def is_supported_binary_asset(name: str) -> bool:
    lower = name.lower()
    if is_checksum_asset(lower) or "source code" in lower or "source-code" in lower:
        return False
    return lower.endswith(SUPPORTED_SUFFIXES)


def parse_checksum_text(content: str) -> dict[str, str]:
    checksums: dict[str, str] = {}
    for raw_line in content.splitlines():
        fields = raw_line.strip().split()
        if not fields or not SHA256_RE.fullmatch(fields[0]):
            continue
        if len(fields) > 1:
            checksums[Path(fields[-1].lstrip("*")).name] = fields[0].lower()
        else:
            checksums[""] = fields[0].lower()
    return checksums


def release_checksums(assets: list[dict], token: str) -> dict[str, str]:
    result: dict[str, str] = {}
    for asset in assets:
        name = str(asset.get("name") or "")
        if not is_checksum_asset(name):
            continue
        url = str(asset.get("browser_download_url") or "")
        if not url:
            continue
        try:
            parsed = parse_checksum_text(request_bytes(url, token="", accept="text/plain").decode("utf-8", "replace"))
        except Exception as error:  # Do not discard a release because an optional checksum file is unavailable.
            print(f"warning: unable to read checksum asset {name}: {error}", file=sys.stderr)
            continue
        lower = name.lower()
        for target, checksum in parsed.items():
            if target:
                result[target] = checksum
            elif lower.endswith((".sha256", ".sha256sum")):
                suffix = ".sha256sum" if lower.endswith(".sha256sum") else ".sha256"
                result[name[: -len(suffix)]] = checksum
    return result


def convert_release(item: dict, token: str) -> dict | None:
    if item.get("draft") or item.get("prerelease"):
        return None
    raw_assets = list(item.get("assets") or [])
    checksums = release_checksums(raw_assets, token)
    assets = []
    for asset in raw_assets:
        name = str(asset.get("name") or "")
        if not is_supported_binary_asset(name):
            continue
        download_url = str(asset.get("browser_download_url") or "")
        if urllib.parse.urlparse(download_url).scheme != "https":
            raise RuntimeError(f"asset {name} has a non-HTTPS download URL")
        assets.append(
            {
                "id": int(asset.get("id") or 0),
                "name": name,
                "size": int(asset.get("size") or 0),
                "downloadUrl": download_url,
                "contentType": str(asset.get("content_type") or ""),
                "publisherSha256": checksums.get(name, ""),
            }
        )
    if not assets:
        return None
    return {
        "id": int(item.get("id") or 0),
        "tagName": str(item.get("tag_name") or ""),
        "name": str(item.get("name") or ""),
        "body": str(item.get("body") or ""),
        "htmlUrl": str(item.get("html_url") or ""),
        "publishedAt": str(item.get("published_at") or ""),
        "prerelease": False,
        "draft": False,
        "assets": assets,
    }


def generate(repository: str, limit: int, token: str) -> dict:
    encoded_repository = urllib.parse.quote(repository, safe="/")
    releases = request_json(f"{API_ROOT}/repos/{encoded_repository}/releases?per_page=30", token)
    if not isinstance(releases, list):
        raise RuntimeError("GitHub Releases response was not a list")
    converted = []
    for item in releases:
        if not isinstance(item, dict):
            continue
        release = convert_release(item, token)
        if release:
            converted.append(release)
        if len(converted) >= limit:
            break
    if not converted:
        raise RuntimeError("upstream returned no stable releases with supported binary assets")
    return {
        "schemaVersion": 1,
        "generatedAt": dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z"),
        "provider": "fingerprint-chromium-static",
        "sourceRepository": repository,
        "releases": converted,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository", default=DEFAULT_REPOSITORY)
    parser.add_argument("--limit", type=int, default=10)
    parser.add_argument("--output", default="browser-core-manifest.json")
    args = parser.parse_args()
    if args.limit < 1 or args.limit > 10:
        parser.error("--limit must be between 1 and 10")
    manifest = generate(args.repository, args.limit, os.environ.get("GITHUB_TOKEN", "").strip())
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {output} with {len(manifest['releases'])} releases")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
