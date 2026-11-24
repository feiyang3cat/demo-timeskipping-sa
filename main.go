package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strconv"
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
		err := fn()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: last attempt: %v", ctx.Err(), err)
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
	cases := []struct {
		name     string
		config   *commonpb.TimeSkippingConfig
		timer    bool
		complete bool
	}{
		{name: "missing"},
		{name: "false", config: &commonpb.TimeSkippingConfig{Enabled: false}},
		{name: "true", config: &commonpb.TimeSkippingConfig{Enabled: true}},
		{name: "skipped-disabled", config: &commonpb.TimeSkippingConfig{Enabled: true, MaxSessionSkipCount: 1}, timer: true},
		{name: "skipped-completed", config: &commonpb.TimeSkippingConfig{Enabled: true}, timer: true, complete: true},
	}
	executions := make([]*commonpb.WorkflowExecution, 0, len(cases))
	for _, tc := range cases {
		id := prefix + "-" + tc.name
		queue := &taskqueuepb.TaskQueue{Name: id + "-workflow", Kind: enumspb.TASK_QUEUE_KIND_NORMAL}
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
		commands := []*commandpb.Command{{
			CommandType: enumspb.COMMAND_TYPE_SCHEDULE_ACTIVITY_TASK,
			Attributes: &commandpb.Command_ScheduleActivityTaskCommandAttributes{
				ScheduleActivityTaskCommandAttributes: &commandpb.ScheduleActivityTaskCommandAttributes{
					ActivityId: "blocked-activity", ActivityType: &commonpb.ActivityType{Name: "IntentionallyUnexecutedActivity"},
					TaskQueue:           &taskqueuepb.TaskQueue{Name: prefix + "-no-activity-worker"},
					StartToCloseTimeout: durationpb.New(24 * time.Hour),
				},
			},
		}}
		if tc.timer {
			commands = []*commandpb.Command{{
				CommandType: enumspb.COMMAND_TYPE_START_TIMER,
				Attributes: &commandpb.Command_StartTimerCommandAttributes{
					StartTimerCommandAttributes: &commandpb.StartTimerCommandAttributes{
						TimerId: "skipped-timer", StartToFireTimeout: durationpb.New(time.Hour),
					},
				},
			}}
		}
		_, err = client.RespondWorkflowTaskCompleted(ctx, &workflowservice.RespondWorkflowTaskCompletedRequest{
			Namespace: *namespace, TaskToken: task.GetTaskToken(), Identity: "time-skipping-sa-demo",
			Commands: commands,
		})
		check(err)
		if tc.complete {
			task, err = client.PollWorkflowTaskQueue(ctx, &workflowservice.PollWorkflowTaskQueueRequest{
				Namespace: *namespace, TaskQueue: queue, Identity: "time-skipping-sa-demo",
			})
			check(err)
			if len(task.GetTaskToken()) == 0 || task.GetWorkflowExecution().GetWorkflowId() != id {
				log.Fatalf("unexpected timer workflow task for %s: %v", id, task.GetWorkflowExecution())
			}
			_, err = client.RespondWorkflowTaskCompleted(ctx, &workflowservice.RespondWorkflowTaskCompletedRequest{
				Namespace: *namespace, TaskToken: task.GetTaskToken(), Identity: "time-skipping-sa-demo",
				Commands: []*commandpb.Command{{CommandType: enumspb.COMMAND_TYPE_COMPLETE_WORKFLOW_EXECUTION,
					Attributes: &commandpb.Command_CompleteWorkflowExecutionCommandAttributes{
						CompleteWorkflowExecutionCommandAttributes: &commandpb.CompleteWorkflowExecutionCommandAttributes{},
					},
				}},
			})
			check(err)
		}
	}
	for i, execution := range executions {
		describe, err := client.DescribeWorkflowExecution(ctx, &workflowservice.DescribeWorkflowExecutionRequest{
			Namespace: *namespace, Execution: execution,
		})
		check(err)
		activities := describe.GetPendingActivities()
		if !cases[i].timer && (len(activities) != 1 || activities[0].GetState() != enumspb.PENDING_ACTIVITY_STATE_SCHEDULED) {
			log.Fatalf("expected one unexecuted scheduled activity for %s: %v", execution.GetWorkflowId(), activities)
		}
		fmt.Printf("Started %s (run %s): workflow=%s\n", execution.GetWorkflowId(), execution.GetRunId(),
			describe.GetWorkflowExecutionInfo().GetStatus())
	}
	query := fmt.Sprintf("WorkflowId STARTS_WITH '%s'", prefix)
	var listed *workflowservice.ListWorkflowExecutionsResponse
	check(retry(ctx, func() error {
		var err error
		listed, err = client.ListWorkflowExecutions(ctx, &workflowservice.ListWorkflowExecutionsRequest{
			Namespace: *namespace, Query: query, PageSize: int32(len(cases) + 1),
		})
		if err != nil {
			return err
		}
		if len(listed.GetExecutions()) != len(cases) {
			return fmt.Errorf("visibility has %d of %d workflows", len(listed.GetExecutions()), len(cases))
		}
		for _, visible := range listed.GetExecutions() {
			id := visible.GetExecution().GetWorkflowId()
			for _, tc := range cases {
				if id != prefix+"-"+tc.name {
					continue
				}
				fields := visible.GetSearchAttributes().GetIndexedFields()
				if tc.config == nil {
					if fields["TemporalTimeSkippingEnabled"] != nil || fields["TemporalTimeSkippingAccumulatedDuration"] != nil {
						return fmt.Errorf("unexpected time-skipping attributes for %s", id)
					}
					continue
				}
				enabled := tc.config.GetEnabled() && tc.config.GetMaxSessionSkipCount() != 1
				if string(fields["TemporalTimeSkippingEnabled"].GetData()) != strconv.FormatBool(enabled) {
					return fmt.Errorf("visibility enabled attribute not updated for %s", id)
				}
				skipped, err := strconv.ParseInt(string(fields["TemporalTimeSkippingAccumulatedDuration"].GetData()), 10, 64)
				if err != nil || (!tc.timer && skipped != 0) || (tc.timer && (skipped <= 0 || skipped > time.Hour.Nanoseconds())) {
					return fmt.Errorf("unexpected accumulated duration for %s: %d ns (%v)", id, skipped, err)
				}
				if tc.complete && visible.GetStatus() != enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED {
					return fmt.Errorf("visibility completion not updated for %s", id)
				}
			}
		}
		return nil
	}))
	for _, visible := range listed.GetExecutions() {
		fields := visible.GetSearchAttributes().GetIndexedFields()
		enabled, accumulated := "missing", "missing"
		if value := fields["TemporalTimeSkippingEnabled"]; value != nil {
			enabled = string(value.GetData())
		}
		if value := fields["TemporalTimeSkippingAccumulatedDuration"]; value != nil {
			accumulated = string(value.GetData()) + " ns"
		}
		fmt.Printf("Visible %s: enabled=%s, accumulatedDuration=%s\n", visible.GetExecution().GetWorkflowId(), enabled, accumulated)
	}
	fmt.Println("Verified enabled and accumulated duration in visibility for all five workflows.")
	fmt.Printf("\nNamespace: %s; gRPC address: %s\n", *namespace, *address)
	fmt.Printf("List this group separately:\n  NAMESPACE=%q ./list-workflows.sh %q\n", *namespace,
		fmt.Sprintf("WorkflowId STARTS_WITH '%s'", prefix))
}
