package conf

import (
	"time"
)

type MyConfType struct {
	PollInterval             time.Duration
	ScaleDownCoolPeriod      time.Duration
	ScaleUpCoolPeriod        time.Duration
	ScaleUpMessages          int
	ScaleDownMessages        int
	MaxPods                  int
	MinPods                  int
	AwsRegion                string
	ScaleUpAmount            float64
	ScaleDownAmount          float64
	ScaleUpOperator          string
	ScaleDownOperator        string
	SqsQueueUrl              string
	KubernetesDeploymentName string
	KubernetesNamespace      string
	ConfigFile               string
	Active                   bool
}
