package core

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// NewCredentials returns a AWS session and and aws.Config ready for use when setting up a new aws service
func NewCredentials(region, roleARN string) (*session.Session, *aws.Config) {
	regionPtr := aws.String(region)
	sess := session.Must(session.NewSession(&aws.Config{Region: regionPtr, CredentialsChainVerboseErrors: aws.Bool(true)}))
	config := &aws.Config{Credentials: stscreds.NewCredentials(sess, roleARN), Region: regionPtr}
	return sess, config
}

// TagValue returns the value of a tag with the name in key from a list of EG2 tags
func TagValue(key string, tags []*ec2.Tag) string {
	for _, tag := range tags {
		if *tag.Key == key && len(*tag.Value) > 0 {
			return *tag.Value
		}
	}
	return ""
}
