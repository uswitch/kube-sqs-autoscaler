package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"os"
	"time"

	"github.com/uswitch/kube-sqs-autoscaler/scale"
	"github.com/uswitch/kube-sqs-autoscaler/sqs"
)

type MyConfType struct {
	pollInterval             time.Duration
	scaleDownCoolPeriod      time.Duration
	scaleUpCoolPeriod        time.Duration
	scaleUpMessages          int
	scaleDownMessages        int
	maxPods                  int
	minPods                  int
	awsRegion                string
	scaleUpAmount            float64
	scaleDownAmount          float64
	scaleUpOperator          string
	scaleDownOperator        string
	sqsQueueUrl              string
	kubernetesDeploymentName string
	kubernetesNamespace      string
	configFile               string
	active                   bool
}

func Run(p *scale.PodAutoScaler, sqs *sqs.SqsClient, myConf MyConfType) {
	var changed bool
	lastScaleUpTime := time.Now()
	lastScaleDownTime := time.Now()

	for {
		log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.kubernetesDeploymentName}).Info("inside polling loop")
		select {
		case <-time.After(myConf.pollInterval):
			{
				numMessages, err := sqs.NumMessages()
				if err != nil {
					log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.kubernetesDeploymentName, "sqs-queue": myConf.sqsQueueUrl, "error": err}).Errorf("Failed to get SQS messages")
					continue
				}

				if numMessages >= myConf.scaleUpMessages {
					if lastScaleUpTime.Add(myConf.scaleUpCoolPeriod).After(time.Now()) {
						log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.kubernetesDeploymentName}).Info("Waiting for cool off, skipping scale up ")
						continue
					}

					log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.kubernetesDeploymentName, "scaleUpMessages": myConf.scaleUpMessages, "numMessages": numMessages}).Info("Scale up may be appropriate")
					if changed, err = p.Scale(scale.UP); err != nil {
						log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.kubernetesDeploymentName}).Errorf("Failed scaling up: %v", err)
						continue
					}
					if changed {
						lastScaleUpTime = time.Now()
					}
				}

				if numMessages <= myConf.scaleDownMessages {
					if lastScaleDownTime.Add(myConf.scaleDownCoolPeriod).After(time.Now()) {
						log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.kubernetesDeploymentName}).Info("Waiting for cool off, skipping scale down")
						continue
					}
					log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.kubernetesDeploymentName, "scaleDownMessages": myConf.scaleDownMessages, "numMessages": numMessages}).Info("Scale down may be appropriate")
					if changed, err = p.Scale(scale.DOWN); err != nil {
						log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.kubernetesDeploymentName}).Errorf("Failed scaling down: %v", err)
						continue
					}
					if changed {
						lastScaleDownTime = time.Now()
					}
				}
			}
		}
	}
}

func main() {
	myConf := MyConfType{}

	flag.DurationVar(&myConf.pollInterval, "poll-period", 30*time.Second, "The interval in seconds for checking if scaling is required")
	flag.DurationVar(&myConf.scaleDownCoolPeriod, "scale-down-cool-off", 30*time.Second, "The cool off period for scaling down")
	flag.DurationVar(&myConf.scaleUpCoolPeriod, "scale-up-cool-off", 120*time.Second, "The cool off period for scaling up")
	flag.IntVar(&myConf.scaleUpMessages, "scale-up-messages", 1000, "Number of sqs messages queued up required for scaling up")
	flag.IntVar(&myConf.scaleDownMessages, "scale-down-messages", 0, "Number of messages required to scale down")
	flag.Float64Var(&myConf.scaleUpAmount, "scale-up-amount", 1, "The number used to scale up the replicas, used with scale-up-operator, e.g. + 3 or * 2")
	flag.Float64Var(&myConf.scaleDownAmount, "scale-down-amount", 1, "The number used to scale down the replicas, used with scale-down-operator, e.g. - 3 or / 2")
	flag.StringVar(&myConf.scaleUpOperator, "scale-up-operator", "+", "The operator used to scale up the replicas, used with scale-up-amount, e.g. + 3 or * 2")
	flag.StringVar(&myConf.scaleDownOperator, "scale-down-operator", "-", "The operator used to scale down the replicas, used with scale-up-amount, e.g. - 3 or / 2")

	flag.IntVar(&myConf.maxPods, "max-pods", 5, "Max pods that kube-sqs-autoscaler can scale")
	flag.IntVar(&myConf.minPods, "min-pods", 1, "Min pods that kube-sqs-autoscaler can scale")
	flag.StringVar(&myConf.awsRegion, "aws-region", "", "Your AWS region")

	flag.StringVar(&myConf.sqsQueueUrl, "sqs-queue-url", "", "The sqs queue url")
	flag.StringVar(&myConf.kubernetesDeploymentName, "kubernetes-deployment", "", "Kubernetes Deployment to scale. This field is required")
	flag.StringVar(&myConf.kubernetesNamespace, "kubernetes-namespace", "default", "The namespace your deployment is running in")

	flag.BoolVar(&myConf.active, "active", true, "true/false - whether autoscaling is active for this deployment. Containers with active=false will terminate with success status")
	flag.Parse()

	if myConf.kubernetesDeploymentName == "" {
		log.Infof("kubernetes-deployment name not set")
		return
	}
	if myConf.sqsQueueUrl == "" {
		log.Infof("sqs-queue-url name not set")
		return
	}
	if !myConf.active {
		log.Infof("active flag set to false, exiting")
		return
	}

	if myConf.scaleDownOperator != "*" && myConf.scaleDownOperator != "/" && myConf.scaleDownOperator != "+" && myConf.scaleDownOperator != "-" {
		log.Infof("scale-down-operator flag %v not in the valid set of *, +, /, - ", myConf.scaleDownOperator)
		os.Exit(1)
	}
	if myConf.scaleUpOperator != "*" && myConf.scaleUpOperator != "/" && myConf.scaleUpOperator != "+" && myConf.scaleUpOperator != "-" {
		log.Infof("scale-up-operator flag %v not in the valid set of *, +, /, - ", myConf.scaleUpOperator)
		os.Exit(1)
	}

	log.Info("Starting kube-sqs-autoscaler for deployment " + myConf.kubernetesDeploymentName + " and namespace " + myConf.kubernetesNamespace)
	log.Infof("Config = %+v ", myConf)
	p := scale.NewPodAutoScaler(myConf.kubernetesDeploymentName, myConf.kubernetesNamespace, myConf.maxPods, myConf.minPods, myConf.scaleUpOperator, myConf.scaleUpAmount, myConf.scaleDownOperator, myConf.scaleDownAmount)
	sqs := sqs.NewSqsClient(myConf.sqsQueueUrl, myConf.awsRegion)

	Run(p, sqs, myConf)

}
