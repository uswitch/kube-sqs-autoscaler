# kube-sqs-autoscaler

Kubernetes pod autoscaler based on queue size in AWS SQS. It periodically retrieves the number of messages in your queue and scales pods accordingly.

This is uswitch's fork of https://github.com/Wattpad/kube-sqs-autoscaler

See https://github.com/uswitch/crm-kub-autoscaler for examples of how this tool can be used.

### Overview

Once per poll-period the system will poll the length of the AWS queue. If the number is above the scale-up threshold then this component triggers a scale up of the number of replicas. If the number is below scale-down threshold then a scale down is triggered. A cool-off period occurs after each scaling to allow the new number of replicas to handle the traffic before scaling is tried again.

The scaling operations change the number of replicas of a kubernetes deployment. The default scaling operation is to add/remove one pod, but other operations can be defined, e.g. scale up can double the number of replicas for a rapid response to increased traffic.

In all cases the resulting number of replicas are restricted to the range (min-pods, max-pods).

The active=false flag can be used to disable a configuration while leaving all the parameters in place. 

### Usage guide
    ./kube-sqs-autoscaler:
    -active
    true/false - whether autoscaling is active for this deployment. Containers with active=false will not monitor queues
    -aws-region string
    Your AWS region
    -kubernetes-deployment string
    Kubernetes Deployment to scale. This field is required
    -kubernetes-namespace string
    The namespace your deployment is running in (default "default")
    -max-pods int
    Max pods that kube-sqs-autoscaler can scale (default 5)
    -min-pods int
    Min pods that kube-sqs-autoscaler can scale (default 1)
    -poll-period duration
    The interval in seconds for checking if scaling is required (default 30s)
    -scale-down-amount float
    The number used to scale down the replicas, used with scale-down-operator, e.g. - 3 or / 2 (default 1)
    -scale-down-cool-off duration
    The cool off period for scaling down (default 30s)
    -scale-down-messages int
    Number of messages required to scale down
    -scale-down-operator string
    The operator used to scale down the replicas, used with scale-up-amount, e.g. - 3 or / 2 (default "-")
    -scale-up-amount float
    The number used to scale up the replicas, used with scale-up-operator, e.g. + 3 or * 2 (default 1)
    -scale-up-cool-off duration
    The cool off period for scaling up (default 2m0s)
    -scale-up-messages int
    Number of sqs messages queued up required for scaling up (default 1000)
    -scale-up-operator string
    The operator used to scale up the replicas, used with scale-up-amount, e.g. + 3 or * 2 (default "+")
    -sqs-queue-url string
    The sqs queue url

### Example

    ./kube-sqs-autoscaler
            --sqs-queue-url=https://sqs.eu-west-1.amazonaws.com/136393635417/crm-firehose-production
            --kubernetes-deployment=crm-firehose-go-production
            --kubernetes-namespace=crm
            --aws-region=eu-west-1
            --poll-period=30s
            --scale-down-cool-off=2m
            --scale-up-cool-off=2m
            --scale-up-messages=50
            --scale-down-messages=10
            --max-pods=10
            --min-pods=1
            --active=true


