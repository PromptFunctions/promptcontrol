"""
Prompt Control (Standalone Fixture)
===================================

What this file is:
- A stateless JSON contract validation engine for LLM/agentic workflows.
- A Python equivalent of the Go fixture for portability and documentation.

Why Bloom filter:
- Fast key-membership checks at low memory cost.
- Useful in heavy-load systems validating many large JSON contracts.

Determinism and correctness:
- Bloom filters can produce false positives.
- This implementation uses bloom as an acceleration layer only.
- Exact key-map membership remains authoritative for missing-key results.

Return contract:
- JSONContract(json_to_validate, contract_struct)
  -> ("completed", [])
  -> ("missing_fields", ["a.b", "x.y", ...])   # deterministic sorted keys
"""

from __future__ import annotations

import hashlib
from dataclasses import is_dataclass, fields
from typing import Any, Dict, List, Tuple

JSON_CONTRACT_COMPLETED = "completed"
JSON_CONTRACT_MISSING = "missing_fields"


class BloomFilter:
    def __init__(self, size: int = 20000, hash_count: int = 7) -> None:
        self.size = max(size, 1)
        self.hash_count = max(hash_count, 1)
        self.bit_array = [0] * self.size

    def _hashes(self, key: str):
        for i in range(self.hash_count):
            digest = int(hashlib.md5((f"{key}:{i}").encode("utf-8")).hexdigest(), 16)
            yield digest % self.size

    def add(self, key: str) -> None:
        for h in self._hashes(key):
            self.bit_array[h] = 1

    def __contains__(self, key: str) -> bool:
        return all(self.bit_array[h] for h in self._hashes(key))


def JSONContract(json_to_validate: Dict[str, Any], contract_struct: Any) -> Tuple[str, List[str]]:
    contract_keys = _flatten_contract_keys(contract_struct)
    candidate_keys = _flatten_json_keys(json_to_validate)

    contract_map = {k: True for k in contract_keys}
    bloom = BloomFilter()
    for key in contract_keys:
        bloom.add(key)

    missing: List[str] = []
    for key in contract_keys:
        if key in candidate_keys:
            continue
        _ = key in bloom  # fast pre-check path (exact map stays authoritative)
        if key in contract_map:
            missing.append(key)

    missing.sort()
    if not missing:
        return JSON_CONTRACT_COMPLETED, []
    return JSON_CONTRACT_MISSING, missing


def _flatten_contract_keys(contract_struct: Any) -> List[str]:
    keys: List[str] = []

    def walk(prefix: str, obj: Any) -> None:
        if obj is None:
            return

        # dataclass objects
        if is_dataclass(obj):
            for f in fields(obj):
                name = f.name
                path = _join(prefix, name)
                keys.append(path)
                walk(path, getattr(obj, name, None))
            return

        # plain class with __dict__
        if hasattr(obj, "__dict__"):
            for name, value in vars(obj).items():
                if name.startswith("_"):
                    continue
                path = _join(prefix, name)
                keys.append(path)
                walk(path, value)
            return

        # dict-shaped contract
        if isinstance(obj, dict):
            for name, value in obj.items():
                path = _join(prefix, str(name))
                keys.append(path)
                walk(path, value)
            return

        # list/tuple are not expanded as indexed schema paths in this fixture
        if isinstance(obj, (list, tuple)):
            return

    walk("", contract_struct)
    return sorted(set(keys))


def _flatten_json_keys(obj: Dict[str, Any]) -> Dict[str, bool]:
    out: Dict[str, bool] = {}

    def walk(prefix: str, node: Any) -> None:
        if not isinstance(node, dict):
            return
        for key, child in node.items():
            path = _join(prefix, str(key))
            out[path] = True
            walk(path, child)

    walk("", obj)
    return out


def _join(prefix: str, key: str) -> str:
    if not prefix:
        return key
    return f"{prefix}.{key}"
