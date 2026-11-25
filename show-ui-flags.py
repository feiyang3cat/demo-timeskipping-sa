#!/usr/bin/env python3
"""Read an HTTP List response from stdin and show the Web UI flag decision."""

import base64
import json
import sys

response = json.load(sys.stdin)
rows = []
for workflow in response.get("executions", []):
    fields = workflow.get("searchAttributes", {}).get("indexedFields", {})
    payload = fields.get("TimeSkippingEnabled")
    enabled = None
    if payload is not None:
        enabled = json.loads(base64.b64decode(payload["data"]))
        if not isinstance(enabled, bool):
            raise ValueError("TimeSkippingEnabled must decode to a Boolean")
    rows.append({
        "workflowId": workflow["execution"]["workflowId"],
        "timeSkippingEnabled": enabled,
        "showTimeSkippingFlag": enabled is True,
    })
json.dump(rows, sys.stdout, indent=2)
print()
