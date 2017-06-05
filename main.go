package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"os"
	"time"

	conf "github.com/uswitch/kube-sqs-autoscaler/conf"
	"github.com/uswitch/kube-sqs-autoscaler/scale"
	"github.com/uswitch/kube-sqs-autoscaler/sqs"
)

func Run(p *scale.PodAutoScaler, sqs *sqs.SqsClient, myConf conf.MyConfType) {
	var changed bool
	lastScaleUpTime := time.Now()
	lastScaleDownTime := time.Now()

	for {
		log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.KubernetesDeploymentName}).Info("inside polling loop")
		select {
		case <-time.After(myConf.PollInterval):
			{
				numMessages, err := sqs.NumMessages()
				if err != nil {
					log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.KubernetesDeploymentName, "sqs-queue": myConf.SqsQueueUrl, "error": err}).Errorf("Failed to get SQS messages")
					continue
				}

				if numMessages >= myConf.ScaleUpMessages {
					if lastScaleUpTime.Add(myConf.ScaleUpCoolPeriod).After(time.Now()) {
						log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.KubernetesDeploymentName}).Info("Waiting for cool off, skipping scale up ")
						continue
					}

					log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.KubernetesDeploymentName, "scaleUpMessages": myConf.ScaleUpMessages, "numMessages": numMessages}).Info("Queue size above threshold, scale up may be appropriate, will check replica count next - scaling will only occur if current replicas below maxPods")
					if changed, err = p.Scale(scale.UP); err != nil {
						log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.KubernetesDeploymentName}).Errorf("Failed scaling up: %v", err)
						continue
					}
					if changed {
						lastScaleUpTime = time.Now()
					}
				}

				if numMessages <= myConf.ScaleDownMessages {
					if lastScaleDownTime.Add(myConf.ScaleDownCoolPeriod).After(time.Now()) {
						log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.KubernetesDeploymentName}).Info("Waiting for cool off, skipping scale down")
						continue
					}
					log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.KubernetesDeploymentName, "scaleDownMessages": myConf.ScaleDownMessages, "numMessages": numMessages}).Info("Queue size below threshold, scale down may be appropriate, will check replica count next  - scaling will only occur if current replicas above minPods")
					if changed, err = p.Scale(scale.DOWN); err != nil {
						log.WithFields(log.Fields{"kubernetesDeploymentName": myConf.KubernetesDeploymentName}).Errorf("Failed scaling down: %v", err)
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
	myConf := conf.MyConfType{}

	flag.DurationVar(&myConf.PollInterval, "poll-period", 30*time.Second, "The interval in seconds for checking if scaling is required")
	flag.DurationVar(&myConf.ScaleDownCoolPeriod, "scale-down-cool-off", 30*time.Second, "The cool off period for scaling down")
	flag.DurationVar(&myConf.ScaleUpCoolPeriod, "scale-up-cool-off", 120*time.Second, "The cool off period for scaling up")
	flag.IntVar(&myConf.ScaleUpMessages, "scale-up-messages", 1000, "Number of sqs messages queued up required for scaling up")
	flag.IntVar(&myConf.ScaleDownMessages, "scale-down-messages", 0, "Number of messages required to scale down")
	flag.Float64Var(&myConf.ScaleUpAmount, "scale-up-amount", 1, "The number used to scale up the replicas, used with scale-up-operator, e.g. + 3 or * 2")
	flag.Float64Var(&myConf.ScaleDownAmount, "scale-down-amount", 1, "The number used to scale down the replicas, used with scale-down-operator, e.g. - 3 or / 2")
	flag.StringVar(&myConf.ScaleUpOperator, "scale-up-operator", "+", "The operator used to scale up the replicas, used with scale-up-amount, e.g. + 3 or * 2")
	flag.StringVar(&myConf.ScaleDownOperator, "scale-down-operator", "-", "The operator used to scale down the replicas, used with scale-up-amount, e.g. - 3 or / 2")

	flag.IntVar(&myConf.MaxPods, "max-pods", 5, "Max pods that kube-sqs-autoscaler can scale")
	flag.IntVar(&myConf.MinPods, "min-pods", 1, "Min pods that kube-sqs-autoscaler can scale")
	flag.StringVar(&myConf.AwsRegion, "aws-region", "", "Your AWS region")

	flag.StringVar(&myConf.SqsQueueUrl, "sqs-queue-url", "", "The sqs queue url")
	flag.StringVar(&myConf.KubernetesDeploymentName, "kubernetes-deployment", "", "Kubernetes Deployment to scale. This field is required")
	flag.StringVar(&myConf.KubernetesNamespace, "kubernetes-namespace", "default", "The namespace your deployment is running in")

	flag.BoolVar(&myConf.Active, "active", true, "true/false - whether autoscaling is active for this deployment. Containers with active=false will terminate with success status")
	flag.Parse()

	if !myConf.Active {
		log.Infof("active flag set to false, will not monitor queue")
		select {}
		// keep active in kubernetes - sleep forever
	}

	if myConf.KubernetesDeploymentName == "" {
		log.Infof("kubernetes-deployment name not set")
		os.Exit(1)
	}
	if myConf.SqsQueueUrl == "" {
		log.Infof("sqs-queue-url name not set")
		os.Exit(1)
	}
	if myConf.ScaleDownOperator != "*" && myConf.ScaleDownOperator != "/" && myConf.ScaleDownOperator != "+" && myConf.ScaleDownOperator != "-" {
		log.Infof("scale-down-operator flag %v not in the valid set of *, +, /, - ", myConf.ScaleDownOperator)
		os.Exit(1)
	}
	if myConf.ScaleUpOperator != "*" && myConf.ScaleUpOperator != "/" && myConf.ScaleUpOperator != "+" && myConf.ScaleUpOperator != "-" {
		log.Infof("scale-up-operator flag %v not in the valid set of *, +, /, - ", myConf.ScaleUpOperator)
		os.Exit(1)
	}

	log.Info("Starting kube-sqs-autoscaler for deployment " + myConf.KubernetesDeploymentName + " and namespace " + myConf.KubernetesNamespace)
	log.Infof("Config = %+v ", myConf)
	p := scale.NewPodAutoScaler(myConf)
	sqs := sqs.NewSqsClient(myConf.SqsQueueUrl, myConf.AwsRegion)

	Run(p, sqs, myConf)

}
