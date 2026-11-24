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

After server changes, restart the demo server to recreate its in-memory SQLite schema.

## Create the examples

In another terminal:

```bash
cd /path/to/demo-timeskipping-sa
./run-workflows.sh
```

Namespace: `time-skipping-sa-demo`. The program creates five workflows and waits until visibility shows the expected attributes.

| Workflow suffix | Status | Enabled | AccumulatedDuration (nanoseconds) |
| --- | --- | --- | --- |
| `-missing` | Running | missing | missing |
| `-false` | Running | false | 0 |
| `-true` | Running | true | 0 |
| `-skipped-disabled` | Running | false | > 0 |
| `-skipped-completed` | Completed | true | > 0 |

The first three workflows retain an unexecuted activity. The last two skip a one-hour timer. `-skipped-disabled` reaches `MaxSessionSkipCount = 1`, which disables time skipping; its timer-fired workflow task remains unhandled. `-skipped-completed` handles that task and completes. Each workflow has its own task queue.

`TemporalTimeSkippingAccumulatedDuration` is an Int search attribute measured in **nanoseconds**, including inherited skipped duration. It is carried by the existing start, enabled-to-disabled, and close visibility writes; individual skips do not trigger extra visibility writes. Other visibility upserts also carry the latest total. The recorded skip can be slightly less than one hour because wall-clock time elapsed before skipping.

The enabled attribute reflects the configuration, so a completed workflow can still have `TemporalTimeSkippingEnabled = true`.

## Search via HTTP List API

```bash
# List running workflows; returns the raw HTTP JSON response.
./list-workflows.sh

# true
./list-workflows.sh "TemporalTimeSkippingEnabled = true"

# Disabled config: matches -false and -skipped-disabled, excluding missing.
./list-workflows.sh "TemporalTimeSkippingEnabled = false"

# missing only
./list-workflows.sh "TemporalTimeSkippingEnabled IS NULL"

# false or missing
./list-workflows.sh "TemporalTimeSkippingEnabled = false OR TemporalTimeSkippingEnabled IS NULL"

# Nonzero skipped duration: both timer examples, including the completed one.
./list-workflows.sh "TemporalTimeSkippingAccumulatedDuration > 0"

# Disabled after actually skipping time.
./list-workflows.sh "TemporalTimeSkippingEnabled = false AND TemporalTimeSkippingAccumulatedDuration > 0"

# Completed with a recorded duration.
./list-workflows.sh "ExecutionStatus = 'Completed' AND TemporalTimeSkippingAccumulatedDuration > 0"
```

To list only the latest five workflows, use the group query printed by `run-workflows.sh`:

```bash
./list-workflows.sh "WorkflowId STARTS_WITH 'sa-demo-<printed-prefix>'"
```

HTTP endpoint: `GET http://127.0.0.1:7243/api/v1/namespaces/time-skipping-sa-demo/workflows`.

## Raw response example

Illustrative execution object for the `-true` example from `ListWorkflowExecutionsResponse.executions`:

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
      "TemporalTimeSkippingAccumulatedDuration": {
        "metadata": {
          "encoding": "anNvbi9wbGFpbg==",
          "type": "SW50"
        },
        "data": "MA=="
      }
    }
  },
  "taskQueue": "sa-demo-1795544555134238000-true-workflow",
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
| enabled `data` on `-true` | `dHJ1ZQ==` | `true` |
| enabled `data` on `-false` | `ZmFsc2U=` | `false` |
| duration `metadata.type` | `SW50` | `Int` |
| duration `data` on `-true` | `MA==` | `0` nanoseconds |

On `-missing`, both time-skipping search attribute entries are absent.

## Temporal CLI and tdbg use cases

### Temporal CLI

```bash
# Enabled config: matches -true and -skipped-completed.
temporal --address 127.0.0.1:7233 --namespace time-skipping-sa-demo workflow list \
  --query 'TemporalTimeSkippingEnabled = true'

# Actually skipped time: matches both timer examples.
temporal --address 127.0.0.1:7233 --namespace time-skipping-sa-demo workflow list \
  --query 'TemporalTimeSkippingAccumulatedDuration > 0'
```

### tdbg

Use a tdbg build that includes `visibility list`.

```bash
# Enabled config: matches -true and -skipped-completed.
tdbg --address 127.0.0.1:7233 --namespace time-skipping-sa-demo visibility list \
  --query 'TemporalTimeSkippingEnabled = true'

# Actually skipped time: matches both timer examples.
tdbg --address 127.0.0.1:7233 --namespace time-skipping-sa-demo visibility list \
  --query 'TemporalTimeSkippingAccumulatedDuration > 0'
```

Both commands accept the same filters as the HTTP examples above: `TemporalTimeSkippingEnabled = false` matches `-false` and `-skipped-disabled`; `TemporalTimeSkippingEnabled IS NULL` matches only `-missing`.

To find workflows that skipped time and are now disabled, use:

```sql
TemporalTimeSkippingAccumulatedDuration > 0 AND TemporalTimeSkippingEnabled = false
```

This matches `-skipped-disabled`. A zero accumulated duration does not indicate whether time skipping was enabled in the past.

### Delete one demo group

Use the exact group prefix printed by the program:

```bash
temporal --address 127.0.0.1:7233 \
  --namespace time-skipping-sa-demo workflow delete \
  --query "WorkflowId STARTS_WITH 'sa-demo-<printed-prefix>'" \
  --reason 'Local demo cleanup' --yes
```

This starts an asynchronous batch deletion for that demo group. Check progress using the printed job ID:

```bash
temporal --address 127.0.0.1:7233 --namespace time-skipping-sa-demo \
  batch describe --job-id <job-id>
```
