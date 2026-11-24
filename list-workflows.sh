#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" || "$#" -gt 1 ]]; then
    printf '%s\n' 'Usage: ./list-workflows.sh ["Temporal list filter"]' \
        'Environment: HTTP_ADDRESS (default http://127.0.0.1:7243), NAMESPACE (default time-skipping-sa-demo),' \
        '             PAGE_SIZE (default 100), NEXT_PAGE_TOKEN, ACCEPT (default application/json+no-payload-shorthand).' \
        'Prints the HTTP response body unchanged. Does not create or update workflows.'
    exit 0
fi
http_address="${HTTP_ADDRESS:-http://127.0.0.1:7243}"
namespace="${NAMESPACE:-time-skipping-sa-demo}"
query="${1:-ExecutionStatus = 'Running'}"
exec curl --silent --show-error --fail-with-body --max-time 30 --get \
    "${http_address%/}/api/v1/namespaces/${namespace}/workflows" \
    --header "Accept: ${ACCEPT:-application/json+no-payload-shorthand}" \
    --data-urlencode "query=$query" \
    --data-urlencode "pageSize=${PAGE_SIZE:-100}" \
    --data-urlencode "nextPageToken=${NEXT_PAGE_TOKEN:-}"
