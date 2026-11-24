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

## Temporal CLI and tdbg use cases

### Temporal CLI

```bash
# Currently enabled: matches the -true example.
temporal --address 127.0.0.1:7233 --namespace time-skipping-sa-demo workflow list \
  --query 'TemporalTimeSkippingEnabled = true'

# Ever enabled: matches the -true example.
temporal --address 127.0.0.1:7233 --namespace time-skipping-sa-demo workflow list \
  --query 'TemporalTimeSkippingOnceEnabled = true'
```

### tdbg

Use a tdbg build that includes `visibility list`.

```bash
# Currently enabled: matches the -true example.
tdbg --address 127.0.0.1:7233 --namespace time-skipping-sa-demo visibility list \
  --query 'TemporalTimeSkippingEnabled = true'

# Ever enabled: matches the -true example.
tdbg --address 127.0.0.1:7233 --namespace time-skipping-sa-demo visibility list \
  --query 'TemporalTimeSkippingOnceEnabled = true'
```

Both commands accept the same filters as the HTTP examples above: `= false` matches only `-false`; `IS NULL` matches only `-missing`.

To find workflows that were enabled and are now disabled, use:

```sql
TemporalTimeSkippingOnceEnabled = true AND TemporalTimeSkippingEnabled = false
```

The three demo workflows do not include this last case, so it returns no matches.

### Delete workflows with time skipping once enabled

```bash
temporal --address 127.0.0.1:7233 \
  --namespace time-skipping-sa-demo workflow delete \
  --query 'TemporalTimeSkippingOnceEnabled = true' \
  --reason 'Local demo cleanup' --yes
```

This starts an asynchronous batch deletion of workflows that have ever had time skipping enabled, including those now disabled. In this demo, it deletes the `-true` example and keeps `-false` and `-missing`.

Check progress using the job ID printed by the command:

```bash
temporal --address 127.0.0.1:7233 --namespace time-skipping-sa-demo \
  batch describe --job-id <job-id>
```
