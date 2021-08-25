package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Printf("! %+v\n", err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	fs := flag.NewFlagSet(argv[0], flag.ContinueOnError)
	var (
		arns   ArnList
		region string
	)
	fs.Var(&arns, "arn", "ARNs")
	fs.StringVar(&region, "region", "", "default region")
	err := fs.Parse(argv[1:])
	if err == flag.ErrHelp {
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("arns=%#v\n", arns)
	var (
		ecsServiceArns []arn.ARN
	)
	for _, a := range arns {
		switch a.Service {
		case "ecs":
			ecsServiceArns = append(ecsServiceArns, a)
		}
	}
	fmt.Printf("ecs services=%#v\n", ecsServiceArns)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var optFns []func(*config.LoadOptions) error
	if region != "" {
		optFns = append(optFns, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return fmt.Errorf("cannot load AWS config: %w", err)
	}
	if err := onECSService(ctx, cfg); err != nil {
		return err
	}
	return nil
}

func onECSService(ctx context.Context, cfg aws.Config) error {
	client := ecs.NewFromConfig(cfg)
	client.DescribeServices(ctx, &ecs.DescribeServicesInput{})
	return nil
}

type ArnList []arn.ARN

var _ flag.Value = &ArnList{}

func (a *ArnList) Set(v string) error {
	var accum []arn.ARN
	for _, x := range strings.Split(v, ",") {
		parsed, err := arn.Parse(x)
		if err != nil {
			return fmt.Errorf("cannot parse arn (%q): %w", x, err)
		}
		accum = append(accum, parsed)
	}
	*a = accum
	return nil
}

func (a ArnList) String() string {
	buf := new(bytes.Buffer)
	size := len(a)
	for i, v := range a {
		buf.WriteString(v.String())
		if i == size-1 {
			break
		}
		buf.WriteByte(',')
	}
	return buf.String()
}
