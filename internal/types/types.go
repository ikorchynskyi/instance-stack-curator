package types

import (
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Instance Group configuration
type Group struct {
	// The name of the group. Required
	Name *string `validate:"required,gt=0"`

	// Group filters. Required
	Filters []ec2Types.Filter `validate:"required,gt=0,dive,required"`

	// Group instance IDs.
	Instances []ec2Types.Instance `yaml:"-"`
}

// Instance Stack configuration
type Stack struct {
	// The name of the stack. Required
	Name *string `validate:"required,gt=0"`

	// The name of the Region.
	Region *string `validate:"omitempty,gt=0"`

	// IAM Role ARN to be assumed.
	RoleARN *string `yaml:"role-arn" validate:"omitempty,gt=0"`

	// Global Stack filters. Required
	Filters []ec2Types.Filter `validate:"required,gt=0,dive,required"`

	// Stack groups. Required
	Groups []Group `validate:"required,gt=0,dive,required"`
}
