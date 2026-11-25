# Time-skipping search attributes demo

## Download and start the server

```bash
git clone --single-branch --branch tsk-compacted-search-attributes-code \
  https://github.com/temporalio/temporal.git temporal

cd /path/to/demo-timeskipping-sa
TEMPORAL_SOURCE="/absolute/path/to/temporal" ./start-server.sh
```

Restart the demo server after schema changes to recreate its in-memory SQLite database.

## Create the examples

In another terminal:

```bash
cd /path/to/demo-timeskipping-sa
./run-workflows.sh
```

Namespace: `time-skipping-sa-demo`. The program creates six workflows, verifies their visible Boolean attributes, and checks seven List/Count filters.

| Workflow suffix | Status | TimeSkippingEnabled | TimeSkippingHasSkippedDuration |
| --- | --- | --- | --- |
| `-missing` | Running | missing | missing |
| `-false` | Running | false | false |
| `-true` | Running | true | false |
| `-skipped-enabled` | Running | true | true |
| `-skipped-disabled` | Running | false | true |
| `-skipped-completed` | Completed | true | true |

The first three workflows wait on an unexecuted activity. The last three skip a one-hour timer. `-skipped-enabled` remains enabled and running, verifying the first skip updates visibility without a configuration change or workflow completion. `-skipped-disabled` reaches `MaxSessionSkipCount = 1`, which disables skipping; its timer-fired workflow task remains unhandled. `-skipped-completed` handles that task and completes. Each workflow uses its own task queue.

`TimeSkippingEnabled` indicates whether time skipping is currently enabled for the workflow. If it was enabled and is later disabled, either manually or automatically, this value becomes `false`. Workflow completion alone does not disable the configuration, so a completed workflow can still return `true`. `TimeSkippingHasSkippedDuration` is true when the mutable state's accumulated skipped duration is positive, including inherited skipped time. This means the workflow uses a virtual clock ahead of wall-clock time, even if time skipping has since been disabled. The numeric duration is not a visibility search attribute.

## Storage and refresh behavior

Only `TemporalTimeSkippingState` is stored and indexed. Its first character represents enabled, and its second represents has-skipped-duration. `T` means true and `F` means false: `FF`, `FT`, `TF`, or `TT`.

Both public Boolean fields share this Keyword index. The server converts Boolean queries to keyword conditions and decodes the stored state into the two Boolean fields in responses. Users do not need to enter the encoded state.

MySQL and PostgreSQL use `VARCHAR(255)`, matching existing Keyword columns and leaving capacity beyond today's two-character format. Future format changes must update readers and query conversion to support old and new encodings before writers change the layout.

The state refreshes on workflow start, enabled-state changes, the first transition from zero to positive accumulated skipped duration, close, and other visibility writes. Subsequent skips do not trigger extra visibility writes because `TimeSkippingHasSkippedDuration` remains `true`. Visibility updates are asynchronous. Workflows without `TimeSkippingInfo` have no state or Boolean attributes; use `IS NULL` to find them.

## Web UI flag (no time-skipping filter)

The Web UI reads `TimeSkippingEnabled` from each workflow's returned search attributes. It does not need a time-skipping query condition or `TimeSkippingHasSkippedDuration` to show the flag.

```bash
./list-workflows.sh | python3 show-ui-flags.py
```

This lists running workflows without a time-skipping filter. The helper decodes the `TimeSkippingEnabled` payload in the HTTP response and shows the flag only when its Boolean value is `true`. Both `false` and a missing attribute hide the flag; a missing value stays `null` in the output.

For all six workflows from one run, including the completed workflow:

```bash
./list-workflows.sh "WorkflowId STARTS_WITH 'sa-demo-<printed-prefix>'" | python3 show-ui-flags.py
```

Expected flag: shown for `-true`, `-skipped-enabled`, and `-skipped-completed`, hidden for `-missing`, `-false`, and `-skipped-disabled`. The completed workflow still has its time-skipping configuration enabled.

The helper uses the default `application/json+no-payload-shorthand` response format from `list-workflows.sh` and Python 3. Each invocation processes the returned page; use `NEXT_PAGE_TOKEN` to fetch additional pages.

## Search via HTTP List API

```bash
./list-workflows.sh

# Currently enabled: -true, -skipped-enabled, and -skipped-completed.
./list-workflows.sh "TimeSkippingEnabled = true"

# Disabled: -false and -skipped-disabled; missing is excluded.
./list-workflows.sh "TimeSkippingEnabled = false"

# Has actually skipped time: all three timer examples.
./list-workflows.sh "TimeSkippingHasSkippedDuration = true"

# Has no skipped duration: -false and -true; missing is excluded.
./list-workflows.sh "TimeSkippingHasSkippedDuration = false"

# No time-skipping information: -missing.
./list-workflows.sh "TimeSkippingEnabled IS NULL"

# Disabled after actually skipping time: -skipped-disabled.
./list-workflows.sh "TimeSkippingEnabled = false AND TimeSkippingHasSkippedDuration = true"

# Completed after skipping time: -skipped-completed.
./list-workflows.sh "ExecutionStatus = 'Completed' AND TimeSkippingHasSkippedDuration = true"
```

To restrict results to one demo run, add the `WorkflowId STARTS_WITH 'sa-demo-<printed-prefix>'` condition printed by the program.

HTTP endpoint: `GET http://127.0.0.1:7243/api/v1/namespaces/time-skipping-sa-demo/workflows`.

## Temporal CLI and tdbg

List workflows that have skipped time and now have time skipping disabled:

```bash
temporal --address 127.0.0.1:7233 --namespace time-skipping-sa-demo workflow list \
  --query 'TimeSkippingEnabled = false AND TimeSkippingHasSkippedDuration = true'
```

Delete workflows that have skipped time:

```bash
temporal --address 127.0.0.1:7233 --namespace time-skipping-sa-demo workflow delete \
  --query 'TimeSkippingHasSkippedDuration = true' \
  --reason 'Delete workflows that skipped time'
```

Terminate workflows with time skipping enabled:

```bash
temporal --address 127.0.0.1:7233 --namespace time-skipping-sa-demo workflow terminate \
  --query 'TimeSkippingEnabled = true' \
  --reason 'Terminate workflows with time skipping enabled'
```

List workflows that have skipped time with tdbg:

```bash
tdbg --address 127.0.0.1:7233 --namespace time-skipping-sa-demo visibility list \
  --query 'TimeSkippingHasSkippedDuration = true'
```

## Delete one demo group

Use the exact prefix printed by the program:

```bash
temporal --address 127.0.0.1:7233 --namespace time-skipping-sa-demo workflow delete \
  --query "WorkflowId STARTS_WITH 'sa-demo-<printed-prefix>'" \
  --reason 'Local demo cleanup' --yes
```
