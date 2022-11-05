package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/k0kubun/pp/v3"
	"github.com/spf13/cobra"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/jmespath/go-jmespath"

	"github.com/ikorchynskyi/instance-stack-curator/internal/curator"
)

// shutdownCmd represents the shutdown command
var shutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Shutdown instance stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initStack(); err != nil {
			return err
		}

		ctx := context.TODO()
		cfg, err := initAWS()
		if err != nil {
			return err
		}

		ec2Client := ec2.NewFromConfig(cfg)
		autoscalingClient := autoscaling.NewFromConfig(cfg)

		for i := range stack.Groups {
			group := stack.Groups[i]
			filters := append(stack.Filters, group.Filters...)
			filters = append(
				filters,
				ec2Types.Filter{
					Name: aws.String("instance-state-name"),
					Values: []string{
						string(ec2Types.InstanceStateNameRunning),
						string(ec2Types.InstanceStateNameStopped),
					},
				},
			)

			if output, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
				Filters: filters,
			}); err != nil {
				return err
			} else {
				for _, r := range output.Reservations {
					group.Instances = append(group.Instances, r.Instances...)
				}
			}

			if len(group.Instances) == 0 {
				pp.Printf("No instances in instance group %v\n", *group.Name)
				continue
			}

			instanceIds := getGroupInstanceIds(&group)
			if dryRun {
				continue
			}

			if err := curator.PrepareInstanceGroupForShutdown(ctx, autoscalingClient, group); err != nil {
				return err
			}

			if output, err := ec2Client.StopInstances(ctx, &ec2.StopInstancesInput{
				InstanceIds: instanceIds,
			}); err != nil {
				return err
			} else {
				pp.Printf("Instance state changes in instance group %v: %v\n", *group.Name, output.StoppingInstances)
			}

			waiter := ec2.NewInstanceStoppedWaiter(ec2Client, func(o *ec2.InstanceStoppedWaiterOptions) {
				o.LogWaitAttempts = true
				o.MaxDelay = time.Minute
			})
			if output, err := waiter.WaitForOutput(ctx, &ec2.DescribeInstancesInput{
				InstanceIds: instanceIds,
			}, curator.DefaultWaitDuration); err != nil {
				return err
			} else {
				pathValue, err := jmespath.Search(
					fmt.Sprintf(
						"Reservations[].Instances[].{%[1]v:%[1]v,%[2]v:%[2]v,%[3]v:%[3]v,%[4]v:%[4]v}",
						"InstanceId",
						"State",
						"StateReason",
						"StateTransitionReason",
					),
					output,
				)
				if err != nil {
					return fmt.Errorf("error evaluating instance state: %w", err)
				}

				listOfValues, ok := pathValue.([]interface{})
				if !ok {
					return fmt.Errorf("expected list got %T", pathValue)
				}
				pp.Printf("Instance states in instance group %v: %v\n", *group.Name, listOfValues)
			}

			pp.Printf("Instance group %v: shutdown has been completed\n", *group.Name)
		}

		pp.Printf("Instance stack %v: shutdown has been completed\n", *stack.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(shutdownCmd)

	// Local flags which will only run when this command is called directly
	shutdownCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Set to true to disable actual instance changes")
}
