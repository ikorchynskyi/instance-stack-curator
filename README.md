# instance-stack-curator

A CLI application to curate an ASG based stacks of EC2 instances.
It allows to execute startup and shutdown of groups of EC2 instances in a predicted sequentional manner.

## Stack configuration example

```yaml
name: stack.name
region: us-west-2
role-arn: arn:aws:iam::account:role/role-name-with-path
filters:
  - name: tag-key
    values:
      - aws:autoscaling:groupName
groups:
  - name: frontend-group
    filters:
      - name: tag:instance-group
        values:
          - frontend
  - name: middleware-group
    filters:
      - name: tag:instance-group
        values:
          - middleware
  - name: backend-group
    filters:
      - name: tag:instance-group
        values:
          - backend
```

For available filter configurations please check [describe-instances](https://docs.aws.amazon.com/cli/latest/reference/ec2/describe-instances.html#options) API
