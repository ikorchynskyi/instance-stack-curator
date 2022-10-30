package cmd

import (
	"context"
	"time"

	"github.com/k0kubun/pp/v3"
	"github.com/spf13/cobra"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

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
				ec2types.Filter{
					Name: aws.String("instance-state-name"),
					Values: []string{
						string(ec2types.InstanceStateNameRunning),
						string(ec2types.InstanceStateNameStopped),
					},
				},
			)

			if output, err := ec2Client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
				Filters: filters,
			}); err != nil {
				return err
			} else {
				for _, r := range output.Reservations {
					for _, i := range r.Instances {
						group.InstanceIds = append(group.InstanceIds, *i.InstanceId)
					}
				}
			}

			if len(group.InstanceIds) == 0 {
				pp.Printf("No instances in instance group %v\n", *group.Name)
				continue
			}
			pp.Printf("Instances in instance group %v: %v\n", *group.Name, group.InstanceIds)

			if output, err := ec2Client.StartInstances(context.TODO(), &ec2.StartInstancesInput{
				InstanceIds: group.InstanceIds,
			}); err != nil {
				return err
			} else {
				pp.Printf("Instance state changes in instance group %v: %v\n", *group.Name, output.StartingInstances)
			}

			waiter := ec2.NewInstanceStatusOkWaiter(ec2Client, func(o *ec2.InstanceStatusOkWaiterOptions) {
				o.LogWaitAttempts = true
				o.MaxDelay = time.Minute
			})
			if output, err := waiter.WaitForOutput(context.TODO(), &ec2.DescribeInstanceStatusInput{
				InstanceIds: group.InstanceIds,
			}, curator.DefaultWaitDuration); err != nil {
				return err
			} else {
				pp.Printf("Instance statuses in instance group %v: %v\n", *group.Name, output.InstanceStatuses)
			}

			curator.PrepareInstanceGroupForStartup(autoscalingClient, group)

			pp.Printf("Instance group %v: startup has been completed\n", *group.Name)
		}

		pp.Printf("Instance stack %v: startup has been completed\n", *stack.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startupCmd)
}
