package validator

import (
	"fmt"

	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/go-playground/validator/v10"

	"github.com/ikorchynskyi/instance-stack-curator/internal/types"
)

var validate *validator.Validate

func FilterStructLevelValidation(sl validator.StructLevel) {
	filter := sl.Current().Interface().(ec2Types.Filter)

	if filter.Name == nil || len(*filter.Name) == 0 {
		sl.ReportError(filter.Name, "Name", "", "required", "")
	}

	if len(filter.Values) == 0 {
		sl.ReportError(filter.Values, "Values", "", "required", "")
	}

	for i, value := range filter.Values {
		if len(value) == 0 {
			sl.ReportError(value, fmt.Sprintf("Values[%v]", i), "", "required", "")
		}
	}
}

func ValidateStack(stack *types.Stack) error {
	validate = validator.New()
	validate.RegisterStructValidation(FilterStructLevelValidation, ec2Types.Filter{})
	return validate.Struct(stack)
}
