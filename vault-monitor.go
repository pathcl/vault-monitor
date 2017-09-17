package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/hashicorp/vault/api"
)

var client *api.Client
var sys api.Sys
var cw *cloudwatch.CloudWatch
var vaultName *string
var awsRegion *string
var namespace *string

func main() {
	defaultInterval := 60
	if i := os.Getenv("CHECK_INTERVAL"); i != "" {
		defaultInterval, err := strconv.Atoi(i)
		if err != nil {
			log.Fatal(err)
		}
		defaultInterval = defaultInterval
	}
	interval := flag.Int("interval", defaultInterval,
		"Time interval of how often to run the check (in seconds). "+
		"Overrides the CHECK_INTERVAL environment variable if set. (default: 60)")

	defaultAddress := "https://127.0.0.1:8200"
	if a := os.Getenv("VAULT_ADDR"); a != "" {
		defaultAddress = a
	}
	address := flag.String("address", defaultAddress,
		"The address of the Vault server. "+
		"Overrides the VAULT_ADDR environment variable if set. (default: https://127.0.0.1:8200)")

	defaultVaultName := "Vault"
	if n := os.Getenv("VAULT_NAME"); n != "" {
		defaultVaultName = n
	}
	vaultName = flag.String("name", defaultVaultName,
		"The name of the Vault (cluster). This value will be used as CloudWatch dimension value. "+
		"Overrides the VAULT_NAME environment variable if set. (default: Vault)")

	defaultNamespace := "Vault"
	if r := os.Getenv("METRIC_NAMESPACE"); r != "" {
		defaultNamespace = r
	}
	namespace = flag.String("namespace", defaultNamespace,
		"AWS CloudWatch metric namespace. "+
		"Overrides the METRIC_NAMESPACE environment variable if set. (default: Vault)")

	defaultRegion := "us-east-1"
	if r := os.Getenv("AWS_REGION"); r != "" {
		defaultRegion = r
	}
	awsRegion = flag.String("region", defaultRegion,
		"AWS CloudWatch region. "+
		"Overrides the AWS_REGION environment variable if set. (default: us-east-1)")

	flag.Parse()

	client, err := api.NewClient(nil)
	if err != nil {
		log.Fatal(err)
	}

	err = client.SetAddress(*address)
	if err != nil {
		log.Fatal(err)
	}

	sys = *client.Sys()

	awsSession := session.New()
	awsSession.Config.WithRegion(*awsRegion)
	cw = cloudwatch.New(awsSession)

	fmt.Println("==> Vault Monitor Configuration:")
	fmt.Println("")
	fmt.Printf("\t      Check interval: %d (seconds)\n", *interval)
	fmt.Printf("\t       Vault Address: %s\n", *address)
	fmt.Printf("\t          Vault Name: %s\n", *vaultName)
	fmt.Printf("\tCloudWatch Namespace: %s\n", *namespace)
	fmt.Printf("\t          AWS Region: %s\n", *awsRegion)
	fmt.Println("")

	checkSealStatus()

	doEvery(time.Duration(*interval)*time.Second, checkSealStatus)
}

func doEvery(d time.Duration, f func()) {
	for _ = range time.Tick(d) {
		f()
	}
}

func checkSealStatus() {
	status, err := sys.SealStatus()
	if err != nil {
		log.Fatal(err)
	}

	var count float64
	if status.Sealed {
		count = 1.0
		log.Printf("Vault sealed")
	} else {
		count = 0.0
		log.Printf("Vault unsealed")
	}

	params := &cloudwatch.PutMetricDataInput{
		MetricData: []*cloudwatch.MetricDatum{
			{
				MetricName: aws.String("SealedVaultCount"),
				Dimensions: []*cloudwatch.Dimension{
					{
						Name:  aws.String("Per Vault"),
						Value: aws.String(*vaultName),
					},
				},
				StatisticValues: &cloudwatch.StatisticSet{
					Maximum:     aws.Float64(count),
					Minimum:     aws.Float64(count),
					SampleCount: aws.Float64(1.0),
					Sum:         aws.Float64(count),
				},
				Timestamp: aws.Time(time.Now()),
				Unit:      aws.String("Count"),
			},
		},
		Namespace: aws.String(*namespace),
	}

	_, err = cw.PutMetricData(params)
	if err != nil {
		log.Println(err.Error())
		return
	}
}
