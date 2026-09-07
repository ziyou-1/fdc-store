"""Buyer-visible referral loop.

Red = a regular buyer cannot share an invite code / cannot see referral rewards.
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
POINTS_PANEL = ROOT / "frontend" / "user" / "src" / "views" / "personal" / "PointsPanel.vue"
PUBLIC_CONFIG = ROOT / "internal" / "modules" / "points" / "transport" / "presenter" / "points.go"

failures: list[str] = []


def fail(msg: str) -> None:
    failures.append(msg)


panel = POINTS_PANEL.read_text(encoding="utf-8")
if "PointsReferralCard" not in panel:
    fail("PointsPanel.vue has no PointsReferralCard")

public_cfg = PUBLIC_CONFIG.read_text(encoding="utf-8")
block_start = public_cfg.find("type PublicConfigResp struct")
block_end = public_cfg.find("}", block_start)
block = public_cfg[block_start:block_end]
if "referral_enabled" not in block:
    fail("PublicConfigResp does not expose referral_enabled")

go_tests = [
    ["./internal/modules/points/application", "TestBuildInvite|TestGetInviteZeroUser"],
    ["./internal/modules/points/transport/presenter", "TestNewPublicConfigRespIncludesReferral|TestNewInviteResp"],
]
for pkg, run in go_tests:
    proc = subprocess.run(
        ["go", "test", pkg, "-run", run, "-count=1"],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        fail(f"go test {pkg} -run {run}\n{proc.stdout}\n{proc.stderr}")

print("=== referral-buyer-loop ===")
if failures:
    for item in failures:
        print(f"RED  {item}")
    sys.exit(1)

print("GREEN  buyer can see and share referral invite")
sys.exit(0)
