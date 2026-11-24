# Time-skipping Search Attributes demo

## Download the server branch

```bash
git clone --single-branch --branch tsk-search-attributes-code \
  https://github.com/temporalio/temporal.git temporal
```

## Start the server

```bash
cd /path/to/demo-timeskipping-sa
TEMPORAL_SOURCE="/absolute/path/to/temporal" ./start-server.sh
```

## Create the three examples

In another terminal:

```bash
cd /path/to/demo-timeskipping-sa
./run-workflows.sh
```

Namespace: `time-skipping-sa-demo`. All three workflows stay Running with an unexecuted activity.

| Workflow suffix | Enabled | OnceEnabled |
| --- | --- | --- |
| `-missing` | missing | missing |
| `-false` | false | false |
| `-true` | true | true |

## Search via HTTP List API

```bash
# List running workflows; returns the raw HTTP JSON response.
./list-workflows.sh

# true
./list-workflows.sh "TemporalTimeSkippingEnabled = true"

# false only: does not match missing.
./list-workflows.sh "TemporalTimeSkippingEnabled = false"

# missing only
./list-workflows.sh "TemporalTimeSkippingEnabled IS NULL"

# false or missing
./list-workflows.sh "TemporalTimeSkippingEnabled = false OR TemporalTimeSkippingEnabled IS NULL"

# OnceEnabled
./list-workflows.sh "TemporalTimeSkippingOnceEnabled = true"
```

To list only the latest three workflows, use the group query printed by `run-workflows.sh`:

```bash
./list-workflows.sh "WorkflowId STARTS_WITH 'sa-demo-<printed-prefix>'"
```

HTTP endpoint: `GET http://127.0.0.1:7243/api/v1/namespaces/time-skipping-sa-demo/workflows`.

## Raw response example

One complete execution object from `ListWorkflowExecutionsResponse.executions`:

```json
{
  "execution": {
    "workflowId": "sa-demo-1795544555134238000-true",
    "runId": "01a20ecb-9e8b-77d0-b142-8f4ecda6ca55"
  },
  "type": {
    "name": "TimeSkippingVisibilityDemo"
  },
  "startTime": "2026-11-24T18:22:35.147513Z",
  "status": "WORKFLOW_EXECUTION_STATUS_RUNNING",
  "executionTime": "2026-11-24T18:22:35.147513Z",
  "memo": {},
  "searchAttributes": {
    "indexedFields": {
      "BuildIds": {
        "metadata": {
          "encoding": "anNvbi9wbGFpbg==",
          "type": "S2V5d29yZExpc3Q="
        },
        "data": "WyJ1bnZlcnNpb25lZCJd"
      },
      "TemporalTimeSkippingEnabled": {
        "metadata": {
          "encoding": "anNvbi9wbGFpbg==",
          "type": "Qm9vbA=="
        },
        "data": "dHJ1ZQ=="
      },
      "TemporalTimeSkippingOnceEnabled": {
        "metadata": {
          "encoding": "anNvbi9wbGFpbg==",
          "type": "Qm9vbA=="
        },
        "data": "dHJ1ZQ=="
      }
    }
  },
  "taskQueue": "sa-demo-1795544555134238000-workflow",
  "rootExecution": {
    "workflowId": "sa-demo-1795544555134238000-true",
    "runId": "01a20ecb-9e8b-77d0-b142-8f4ecda6ca55"
  }
}
```

Payload metadata and data are byte fields, represented as Base64 in this HTTP response:

| Field | Raw value | Base64-decoded value |
| --- | --- | --- |
| `metadata.encoding` | `anNvbi9wbGFpbg==` | `json/plain`: the data contains JSON |
| `metadata.type` | `Qm9vbA==` | `Bool`: the search attribute type |
| `data` on `-true` | `dHJ1ZQ==` | `true` |
| `data` on `-false` | `ZmFsc2U=` | `false` |

On `-missing`, the entire `TemporalTimeSkippingEnabled` entry is absent. `OnceEnabled` follows the same three cases in this demo.
