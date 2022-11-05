package cmd

import (
	"context"
	"time"

	"github.com/k0kubun/pp/v3"
	"github.com/spf13/cobra"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/ikorchynskyi/instance-stack-curator/internal/curator"
)

// startupCmd represents the startup command
var startupCmd = &cobra.Command{
	Use:   "startup",
	Short: "Startup instance stack",
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
			group := stack.Groups[len(stack.Groups)-1-i]
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

			if output, err := ec2Client.StartInstances(ctx, &ec2.StartInstancesInput{
				InstanceIds: instanceIds,
			}); err != nil {
				return err
			} else {
				pp.Printf("Instance state changes in instance group %v: %v\n", *group.Name, output.StartingInstances)
			}

			waiter := ec2.NewInstanceStatusOkWaiter(ec2Client, func(o *ec2.InstanceStatusOkWaiterOptions) {
				o.LogWaitAttempts = true
				o.MaxDelay = time.Minute
			})
			if output, err := waiter.WaitForOutput(ctx, &ec2.DescribeInstanceStatusInput{
				InstanceIds: instanceIds,
			}, curator.DefaultWaitDuration); err != nil {
				return err
			} else {
				pp.Printf("Instance statuses in instance group %v: %v\n", *group.Name, output.InstanceStatuses)
			}

			curator.PrepareInstanceGroupForStartup(ctx, autoscalingClient, group)

			pp.Printf("Instance group %v: startup has been completed\n", *group.Name)
		}

		pp.Printf("Instance stack %v: startup has been completed\n", *stack.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startupCmd)

	// Local flags which will only run when this command is called directly
	startupCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Set to true to disable actual instance changes")
}
