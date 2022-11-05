package curator

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/smithy-go/middleware"
	smithytime "github.com/aws/smithy-go/time"
	smithywaiter "github.com/aws/smithy-go/waiter"
	"github.com/jmespath/go-jmespath"
	"github.com/k0kubun/pp/v3"

	"github.com/ikorchynskyi/instance-stack-curator/internal/types"
)

const (
	LifecycleStateNameInService string = "InService"
	LifecycleStateNameStandby   string = "Standby"
)

const (
	DefaultWaitDuration time.Duration = 10 * time.Minute
)

// AutoScalingInstanceStandbyWaiterOptions are waiter options for AutoScalingInstanceStandbyWaiter
type AutoScalingInstanceStandbyWaiterOptions struct {

	// Set of options to modify how an operation is invoked. These apply to all
	// operations invoked for this client. Use functional options on operation call to
	// modify this list for per operation behavior.
	APIOptions []func(*middleware.Stack) error

	// MinDelay is the minimum amount of time to delay between retries. If unset,
	// AutoScalingInstanceStandbyWaiter will use default minimum delay of 15 seconds. Note that
	// MinDelay must resolve to a value lesser than or equal to the MaxDelay.
	MinDelay time.Duration

	// MaxDelay is the maximum amount of time to delay between retries. If unset or set
	// to zero, AutoScalingInstanceStandbyWaiter will use default max delay of 120 seconds. Note
	// that MaxDelay must resolve to value greater than or equal to the MinDelay.
	MaxDelay time.Duration

	// LogWaitAttempts is used to enable logging for waiter retry attempts
	LogWaitAttempts bool

	// Retryable is function that can be used to override the service defined
	// waiter-behavior based on operation output, or returned error. This function is
	// used by the waiter to decide if a state is retryable or a terminal state. By
	// default service-modeled logic will populate this option. This option can thus be
	// used to define a custom waiter state with fall-back to service-modeled waiter
	// state mutators.The function returns an error in case of a failure state. In case
	// of retry state, this function returns a bool value of true and nil error, while
	// in case of success it returns a bool value of false and nil error.
	Retryable func(context.Context, *autoscaling.DescribeAutoScalingInstancesInput, *autoscaling.DescribeAutoScalingInstancesOutput, error) (bool, error)
}

// AutoScalingInstanceStandbyWaiter defines the waiters for AutoScalingInstanceStandby
type AutoScalingInstanceStandbyWaiter struct {
	client autoscaling.DescribeAutoScalingInstancesAPIClient

	options AutoScalingInstanceStandbyWaiterOptions
}

// NewAutoScalingInstanceStandbyWaiter constructs a AutoScalingInstanceStandbyWaiter.
func NewAutoScalingInstanceStandbyWaiter(client autoscaling.DescribeAutoScalingInstancesAPIClient, optFns ...func(*AutoScalingInstanceStandbyWaiterOptions)) *AutoScalingInstanceStandbyWaiter {
	options := AutoScalingInstanceStandbyWaiterOptions{}
	options.MinDelay = 15 * time.Second
	options.MaxDelay = 120 * time.Second
	options.Retryable = autoScalingInstanceStandbyStateRetryable

	for _, fn := range optFns {
		fn(&options)
	}
	return &AutoScalingInstanceStandbyWaiter{
		client:  client,
		options: options,
	}
}

// Wait calls the waiter function for AutoScalingInstanceStandby waiter. The maxWaitDur is the
// maximum wait duration the waiter will wait. The maxWaitDur is required and must
// be greater than zero.
func (w *AutoScalingInstanceStandbyWaiter) Wait(ctx context.Context, params *autoscaling.DescribeAutoScalingInstancesInput, maxWaitDur time.Duration, optFns ...func(*AutoScalingInstanceStandbyWaiterOptions)) error {
	_, err := w.WaitForOutput(ctx, params, maxWaitDur, optFns...)
	return err
}

// WaitForOutput calls the waiter function for AutoScalingInstanceStandby waiter and returns
// the output of the successful operation. The maxWaitDur is the maximum wait
// duration the waiter will wait. The maxWaitDur is required and must be greater
// than zero.
func (w *AutoScalingInstanceStandbyWaiter) WaitForOutput(ctx context.Context, params *autoscaling.DescribeAutoScalingInstancesInput, maxWaitDur time.Duration, optFns ...func(*AutoScalingInstanceStandbyWaiterOptions)) (*autoscaling.DescribeAutoScalingInstancesOutput, error) {
	if maxWaitDur <= 0 {
		return nil, fmt.Errorf("maximum wait time for waiter must be greater than zero")
	}

	options := w.options
	for _, fn := range optFns {
		fn(&options)
	}

	if options.MaxDelay <= 0 {
		options.MaxDelay = 120 * time.Second
	}

	if options.MinDelay > options.MaxDelay {
		return nil, fmt.Errorf("minimum waiter delay %v must be lesser than or equal to maximum waiter delay of %v.", options.MinDelay, options.MaxDelay)
	}

	ctx, cancelFn := context.WithTimeout(ctx, maxWaitDur)
	defer cancelFn()

	logger := smithywaiter.Logger{}
	remainingTime := maxWaitDur

	var attempt int64
	for {

		attempt++
		apiOptions := options.APIOptions
		start := time.Now()

		if options.LogWaitAttempts {
			logger.Attempt = attempt
			apiOptions = append([]func(*middleware.Stack) error{}, options.APIOptions...)
			apiOptions = append(apiOptions, logger.AddLogger)
		}

		out, err := w.client.DescribeAutoScalingInstances(ctx, params, func(o *autoscaling.Options) {
			o.APIOptions = append(o.APIOptions, apiOptions...)
		})

		retryable, err := options.Retryable(ctx, params, out, err)
		if err != nil {
			return nil, err
		}
		if !retryable {
			return out, nil
		}

		remainingTime -= time.Since(start)
		if remainingTime < options.MinDelay || remainingTime <= 0 {
			break
		}

		// compute exponential backoff between waiter retries
		delay, err := smithywaiter.ComputeDelay(
			attempt, options.MinDelay, options.MaxDelay, remainingTime,
		)
		if err != nil {
			return nil, fmt.Errorf("error computing waiter delay, %w", err)
		}

		remainingTime -= delay
		// sleep for the delay amount before invoking a request
		if err := smithytime.SleepWithContext(ctx, delay); err != nil {
			return nil, fmt.Errorf("request cancelled while waiting, %w", err)
		}
	}
	return nil, fmt.Errorf("exceeded max wait time for AutoScalingInstanceStandby waiter")
}

func autoScalingInstanceStandbyStateRetryable(ctx context.Context, input *autoscaling.DescribeAutoScalingInstancesInput, output *autoscaling.DescribeAutoScalingInstancesOutput, err error) (bool, error) {
	if err == nil {
		pathValue, err := jmespath.Search("AutoScalingInstances[].LifecycleState", output)
		if err != nil {
			return false, fmt.Errorf("error evaluating waiter state: %w", err)
		}

		var match = true
		listOfValues, ok := pathValue.([]interface{})
		if !ok {
			return false, fmt.Errorf("waiter comparator expected list got %T", pathValue)
		}

		if len(listOfValues) == 0 {
			match = false
		}
		for _, v := range listOfValues {
			value, ok := v.(*string)
			if !ok {
				return false, fmt.Errorf("waiter comparator expected string value, got %T", pathValue)
			}

			if *value != LifecycleStateNameStandby {
				match = false
			}
		}

		if match {
			return false, nil
		}
	}

	return true, nil
}

// AutoScalingInstanceInServiceWaiterOptions are waiter options for AutoScalingInstanceInServiceWaiter
type AutoScalingInstanceInServiceWaiterOptions struct {

	// Set of options to modify how an operation is invoked. These apply to all
	// operations invoked for this client. Use functional options on operation call to
	// modify this list for per operation behavior.
	APIOptions []func(*middleware.Stack) error

	// MinDelay is the minimum amount of time to delay between retries. If unset,
	// AutoScalingInstanceInServiceWaiter will use default minimum delay of 15 seconds. Note that
	// MinDelay must resolve to a value lesser than or equal to the MaxDelay.
	MinDelay time.Duration

	// MaxDelay is the maximum amount of time to delay between retries. If unset or set
	// to zero, AutoScalingInstanceInServiceWaiter will use default max delay of 120 seconds. Note
	// that MaxDelay must resolve to value greater than or equal to the MinDelay.
	MaxDelay time.Duration

	// LogWaitAttempts is used to enable logging for waiter retry attempts
	LogWaitAttempts bool

	// Retryable is function that can be used to override the service defined
	// waiter-behavior based on operation output, or returned error. This function is
	// used by the waiter to decide if a state is retryable or a terminal state. By
	// default service-modeled logic will populate this option. This option can thus be
	// used to define a custom waiter state with fall-back to service-modeled waiter
	// state mutators.The function returns an error in case of a failure state. In case
	// of retry state, this function returns a bool value of true and nil error, while
	// in case of success it returns a bool value of false and nil error.
	Retryable func(context.Context, *autoscaling.DescribeAutoScalingInstancesInput, *autoscaling.DescribeAutoScalingInstancesOutput, error) (bool, error)
}

// AutoScalingInstanceInServiceWaiter defines the waiters for AutoScalingInstanceInService
type AutoScalingInstanceInServiceWaiter struct {
	client autoscaling.DescribeAutoScalingInstancesAPIClient

	options AutoScalingInstanceInServiceWaiterOptions
}

// NewAutoScalingInstanceInServiceWaiter constructs a AutoScalingInstanceInServiceWaiter.
func NewAutoScalingInstanceInServiceWaiter(client autoscaling.DescribeAutoScalingInstancesAPIClient, optFns ...func(*AutoScalingInstanceInServiceWaiterOptions)) *AutoScalingInstanceInServiceWaiter {
	options := AutoScalingInstanceInServiceWaiterOptions{}
	options.MinDelay = 15 * time.Second
	options.MaxDelay = 120 * time.Second
	options.Retryable = AutoScalingInstanceInServiceStateRetryable

	for _, fn := range optFns {
		fn(&options)
	}
	return &AutoScalingInstanceInServiceWaiter{
		client:  client,
		options: options,
	}
}

// Wait calls the waiter function for AutoScalingInstanceInService waiter. The maxWaitDur is the
// maximum wait duration the waiter will wait. The maxWaitDur is required and must
// be greater than zero.
func (w *AutoScalingInstanceInServiceWaiter) Wait(ctx context.Context, params *autoscaling.DescribeAutoScalingInstancesInput, maxWaitDur time.Duration, optFns ...func(*AutoScalingInstanceInServiceWaiterOptions)) error {
	_, err := w.WaitForOutput(ctx, params, maxWaitDur, optFns...)
	return err
}

// WaitForOutput calls the waiter function for AutoScalingInstanceInService waiter and returns
// the output of the successful operation. The maxWaitDur is the maximum wait
// duration the waiter will wait. The maxWaitDur is required and must be greater
// than zero.
func (w *AutoScalingInstanceInServiceWaiter) WaitForOutput(ctx context.Context, params *autoscaling.DescribeAutoScalingInstancesInput, maxWaitDur time.Duration, optFns ...func(*AutoScalingInstanceInServiceWaiterOptions)) (*autoscaling.DescribeAutoScalingInstancesOutput, error) {
	if maxWaitDur <= 0 {
		return nil, fmt.Errorf("maximum wait time for waiter must be greater than zero")
	}

	options := w.options
	for _, fn := range optFns {
		fn(&options)
	}

	if options.MaxDelay <= 0 {
		options.MaxDelay = 120 * time.Second
	}

	if options.MinDelay > options.MaxDelay {
		return nil, fmt.Errorf("minimum waiter delay %v must be lesser than or equal to maximum waiter delay of %v.", options.MinDelay, options.MaxDelay)
	}

	ctx, cancelFn := context.WithTimeout(ctx, maxWaitDur)
	defer cancelFn()

	logger := smithywaiter.Logger{}
	remainingTime := maxWaitDur

	var attempt int64
	for {

		attempt++
		apiOptions := options.APIOptions
		start := time.Now()

		if options.LogWaitAttempts {
			logger.Attempt = attempt
			apiOptions = append([]func(*middleware.Stack) error{}, options.APIOptions...)
			apiOptions = append(apiOptions, logger.AddLogger)
		}

		out, err := w.client.DescribeAutoScalingInstances(ctx, params, func(o *autoscaling.Options) {
			o.APIOptions = append(o.APIOptions, apiOptions...)
		})

		retryable, err := options.Retryable(ctx, params, out, err)
		if err != nil {
			return nil, err
		}
		if !retryable {
			return out, nil
		}

		remainingTime -= time.Since(start)
		if remainingTime < options.MinDelay || remainingTime <= 0 {
			break
		}

		// compute exponential backoff between waiter retries
		delay, err := smithywaiter.ComputeDelay(
			attempt, options.MinDelay, options.MaxDelay, remainingTime,
		)
		if err != nil {
			return nil, fmt.Errorf("error computing waiter delay, %w", err)
		}

		remainingTime -= delay
		// sleep for the delay amount before invoking a request
		if err := smithytime.SleepWithContext(ctx, delay); err != nil {
			return nil, fmt.Errorf("request cancelled while waiting, %w", err)
		}
	}
	return nil, fmt.Errorf("exceeded max wait time for AutoScalingInstanceInService waiter")
}

func AutoScalingInstanceInServiceStateRetryable(ctx context.Context, input *autoscaling.DescribeAutoScalingInstancesInput, output *autoscaling.DescribeAutoScalingInstancesOutput, err error) (bool, error) {
	if err == nil {
		pathValue, err := jmespath.Search("AutoScalingInstances[].LifecycleState", output)
		if err != nil {
			return false, fmt.Errorf("error evaluating waiter state: %w", err)
		}

		var match = true
		listOfValues, ok := pathValue.([]interface{})
		if !ok {
			return false, fmt.Errorf("waiter comparator expected list got %T", pathValue)
		}

		if len(listOfValues) == 0 {
			match = false
		}
		for _, v := range listOfValues {
			value, ok := v.(*string)
			if !ok {
				return false, fmt.Errorf("waiter comparator expected string value, got %T", pathValue)
			}

			if *value != LifecycleStateNameInService {
				match = false
			}
		}

		if match {
			return false, nil
		}
	}

	return true, nil
}

func PrepareInstanceGroupForShutdown(ctx context.Context, autoscalingClient *autoscaling.Client, group types.Group) error {
	instanceIds := make([]string, 0, len(group.Instances))
	for _, i := range group.Instances {
		instanceIds = append(instanceIds, *i.InstanceId)
	}
	autoScalingInstancesOutput, err := autoscalingClient.DescribeAutoScalingInstances(ctx, &autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: instanceIds,
	})
	if err != nil {
		return err
	}

	autoscalingInstances := make(map[string][]string)
	for _, i := range autoScalingInstancesOutput.AutoScalingInstances {
		// only InService instances may be put into Standby
		if *i.LifecycleState == LifecycleStateNameInService {
			autoscalingInstances[*i.AutoScalingGroupName] = append(autoscalingInstances[*i.AutoScalingGroupName], *i.InstanceId)
		}
	}

	if len(autoscalingInstances) == 0 {
		pp.Printf("No Auto Scaling Groups in instance group %v\n", *group.Name)
		return nil
	}

	asgNames := make([]string, 0, len(autoscalingInstances))
	for k := range autoscalingInstances {
		asgNames = append(asgNames, k)
	}
	pp.Printf("Auto Scaling Groups in instance group %v: %v\n", *group.Name, asgNames)

	describeAutoScalingGroupsOutput, err := autoscalingClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: asgNames,
	})
	if err != nil {
		return err
	}

	waitForInstanceIds := make([]string, 0)
	for _, g := range describeAutoScalingGroupsOutput.AutoScalingGroups {
		instanceIds, ok := autoscalingInstances[*g.AutoScalingGroupName]
		if !ok {
			continue
		}

		// Update ASG(s) MinSize before a putting into standby
		if *g.MinSize > 0 {
			minSize := *g.MinSize - int32(len(instanceIds))
			if minSize < 0 {
				minSize = 0
			}
			_, err := autoscalingClient.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
				AutoScalingGroupName: g.AutoScalingGroupName,
				MinSize:              aws.Int32(minSize),
			})
			if err != nil {
				return err
			}
		}

		enterStandbyOutput, err := autoscalingClient.EnterStandby(ctx, &autoscaling.EnterStandbyInput{
			AutoScalingGroupName:           g.AutoScalingGroupName,
			InstanceIds:                    instanceIds,
			ShouldDecrementDesiredCapacity: aws.Bool(true),
		})
		if err != nil {
			return err
		}

		pp.Printf("Scaling activities in ASG %v: %v\n", *g.AutoScalingGroupName, enterStandbyOutput.Activities)
		waitForInstanceIds = append(waitForInstanceIds, instanceIds...)
	}

	if len(waitForInstanceIds) == 0 {
		return nil
	}
	standbyWaiter := NewAutoScalingInstanceStandbyWaiter(autoscalingClient, func(o *AutoScalingInstanceStandbyWaiterOptions) {
		o.LogWaitAttempts = true
		o.MaxDelay = time.Minute
	})

	if output, err := standbyWaiter.WaitForOutput(ctx, &autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: waitForInstanceIds,
	}, DefaultWaitDuration); err != nil {
		return err
	} else {
		pp.Printf("Auto Scaling instances in instance group %v: %v\n", *group.Name, output.AutoScalingInstances)
	}

	return nil
}

func PrepareInstanceGroupForStartup(ctx context.Context, autoscalingClient *autoscaling.Client, group types.Group) error {
	instanceIds := make([]string, 0, len(group.Instances))
	for _, i := range group.Instances {
		instanceIds = append(instanceIds, *i.InstanceId)
	}
	autoScalingInstancesOutput, err := autoscalingClient.DescribeAutoScalingInstances(ctx, &autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: instanceIds,
	})
	if err != nil {
		return err
	}

	autoscalingInstances := make(map[string][]string)
	for _, i := range autoScalingInstancesOutput.AutoScalingInstances {
		// only Standby instances may be put into InService
		if *i.LifecycleState == LifecycleStateNameStandby {
			autoscalingInstances[*i.AutoScalingGroupName] = append(autoscalingInstances[*i.AutoScalingGroupName], *i.InstanceId)
		}
	}

	if len(autoscalingInstances) == 0 {
		pp.Printf("No Auto Scaling Groups in instance group %v\n", *group.Name)
		return nil
	}

	asgNames := make([]string, 0, len(autoscalingInstances))
	for k := range autoscalingInstances {
		asgNames = append(asgNames, k)
	}
	pp.Printf("Auto Scaling Groups in instance group %v: %v\n", *group.Name, asgNames)

	describeAutoScalingGroupsOutput, err := autoscalingClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: asgNames,
	})
	if err != nil {
		return err
	}

	waitForInstanceIds := make([]string, 0)
	for _, g := range describeAutoScalingGroupsOutput.AutoScalingGroups {
		instanceIds, ok := autoscalingInstances[*g.AutoScalingGroupName]
		if !ok {
			continue
		}

		// Update ASG(s) MaxSize before a returning an instance to service
		if maxSize := int32(len(g.Instances)); *g.MaxSize < maxSize {
			_, err := autoscalingClient.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
				AutoScalingGroupName: g.AutoScalingGroupName,
				MaxSize:              aws.Int32(maxSize),
			})
			if err != nil {
				return err
			}
		}

		exitStandbyOutput, err := autoscalingClient.ExitStandby(ctx, &autoscaling.ExitStandbyInput{
			AutoScalingGroupName: g.AutoScalingGroupName,
			InstanceIds:          instanceIds,
		})
		if err != nil {
			return err
		}

		pp.Printf("Scaling activities in ASG %v: %v\n", *g.AutoScalingGroupName, exitStandbyOutput.Activities)
		waitForInstanceIds = append(waitForInstanceIds, instanceIds...)
	}

	if len(waitForInstanceIds) == 0 {
		return nil
	}
	inServiceWaiter := NewAutoScalingInstanceInServiceWaiter(autoscalingClient, func(o *AutoScalingInstanceInServiceWaiterOptions) {
		o.LogWaitAttempts = true
		o.MaxDelay = time.Minute
	})

	if output, err := inServiceWaiter.WaitForOutput(ctx, &autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: waitForInstanceIds,
	}, DefaultWaitDuration); err != nil {
		return err
	} else {
		pp.Printf("Auto Scaling instances in instance group %v: %v\n", *group.Name, output.AutoScalingInstances)
	}

	// Update ASG(s) MinSize after a returning an instance to service
	for _, g := range describeAutoScalingGroupsOutput.AutoScalingGroups {
		instanceIds, ok := autoscalingInstances[*g.AutoScalingGroupName]
		if !ok {
			continue
		}

		minSize := int32(len(instanceIds))
		if *g.MinSize >= minSize {
			continue
		}

		_, err := autoscalingClient.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
			AutoScalingGroupName: g.AutoScalingGroupName,
			MinSize:              aws.Int32(minSize),
		})
		if err != nil {
			return err
		}
	}

	return nil
}
