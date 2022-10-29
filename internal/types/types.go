package types

import (
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type Group struct {
	Name        *string           `validate:"required,gt=0"`
	Filters     []ec2types.Filter `validate:"required,gt=0,dive,required"`
	InstanceIds []string
}

type Stack struct {
	Name    *string           `validate:"required,gt=0"`
	Filters []ec2types.Filter `validate:"required,gt=0,dive,required"`
	Groups  []Group           `validate:"required,gt=0,dive,required"`
}
