package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	commandpb "go.temporal.io/api/command/v1"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/api/workflowservice/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func retry(ctx context.Context, fn func() error) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := fn(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func main() {
	address := flag.String("address", "127.0.0.1:7233", "local server gRPC address")
	namespace := flag.String("namespace", "time-skipping-sa-demo", "dedicated demo namespace")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(*address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	check(err)
	defer conn.Close()
	client := workflowservice.NewWorkflowServiceClient(conn)
	check(retry(ctx, func() error {
		attempt, done := context.WithTimeout(ctx, 2*time.Second)
		defer done()
		_, err := client.GetSystemInfo(attempt, &workflowservice.GetSystemInfoRequest{})
		return err
	}))
	_, err = client.RegisterNamespace(ctx, &workflowservice.RegisterNamespaceRequest{
		Namespace: *namespace, WorkflowExecutionRetentionPeriod: durationpb.New(24 * time.Hour),
	})
	if status.Code(err) != codes.AlreadyExists {
		check(err)
	}
	prefix := fmt.Sprintf("sa-demo-%d", time.Now().UnixNano())
	queue := &taskqueuepb.TaskQueue{Name: prefix + "-workflow", Kind: enumspb.TASK_QUEUE_KIND_NORMAL}
	cases := []struct {
		name   string
		config *commonpb.TimeSkippingConfig
	}{
		{name: "missing"},
		{name: "false", config: &commonpb.TimeSkippingConfig{Enabled: false}},
		{name: "true", config: &commonpb.TimeSkippingConfig{Enabled: true}},
	}
	executions := make([]*commonpb.WorkflowExecution, 0, len(cases))
	for _, tc := range cases {
		id := prefix + "-" + tc.name
		start, err := client.StartWorkflowExecution(ctx, &workflowservice.StartWorkflowExecutionRequest{
			Namespace: *namespace, RequestId: id, WorkflowId: id,
			WorkflowType: &commonpb.WorkflowType{Name: "TimeSkippingVisibilityDemo"}, TaskQueue: queue,
			WorkflowRunTimeout: durationpb.New(7 * 24 * time.Hour), WorkflowTaskTimeout: durationpb.New(30 * time.Second),
			TimeSkippingConfig: tc.config,
		})
		check(err)
		execution := &commonpb.WorkflowExecution{WorkflowId: id, RunId: start.GetRunId()}
		executions = append(executions, execution)
		task, err := client.PollWorkflowTaskQueue(ctx, &workflowservice.PollWorkflowTaskQueueRequest{
			Namespace: *namespace, TaskQueue: queue, Identity: "time-skipping-sa-demo",
		})
		check(err)
		if len(task.GetTaskToken()) == 0 || task.GetWorkflowExecution().GetWorkflowId() != id {
			log.Fatalf("unexpected workflow task for %s: %v", id, task.GetWorkflowExecution())
		}
		_, err = client.RespondWorkflowTaskCompleted(ctx, &workflowservice.RespondWorkflowTaskCompletedRequest{
			Namespace: *namespace, TaskToken: task.GetTaskToken(), Identity: "time-skipping-sa-demo",
			Commands: []*commandpb.Command{{
				CommandType: enumspb.COMMAND_TYPE_SCHEDULE_ACTIVITY_TASK,
				Attributes: &commandpb.Command_ScheduleActivityTaskCommandAttributes{
					ScheduleActivityTaskCommandAttributes: &commandpb.ScheduleActivityTaskCommandAttributes{
						ActivityId: "blocked-activity", ActivityType: &commonpb.ActivityType{Name: "IntentionallyUnexecutedActivity"},
						TaskQueue:           &taskqueuepb.TaskQueue{Name: prefix + "-no-activity-worker"},
						StartToCloseTimeout: durationpb.New(24 * time.Hour),
					},
				},
			}},
		})
		check(err)
	}
	for _, execution := range executions {
		describe, err := client.DescribeWorkflowExecution(ctx, &workflowservice.DescribeWorkflowExecutionRequest{
			Namespace: *namespace, Execution: execution,
		})
		check(err)
		activities := describe.GetPendingActivities()
		if len(activities) != 1 || activities[0].GetState() != enumspb.PENDING_ACTIVITY_STATE_SCHEDULED {
			log.Fatalf("expected one unexecuted scheduled activity for %s: %v", execution.GetWorkflowId(), activities)
		}
		fmt.Printf("Started %s (run %s): workflow=%s, activity=%s\n", execution.GetWorkflowId(), execution.GetRunId(),
			describe.GetWorkflowExecutionInfo().GetStatus(), activities[0].GetState())
	}
	fmt.Printf("\nNamespace: %s; gRPC address: %s\n", *namespace, *address)
	fmt.Printf("List this group separately:\n  NAMESPACE=%q ./list-workflows.sh %q\n", *namespace,
		fmt.Sprintf("WorkflowId STARTS_WITH '%s'", prefix))
}
