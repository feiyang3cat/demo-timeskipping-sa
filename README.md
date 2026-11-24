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
