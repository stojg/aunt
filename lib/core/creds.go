package core

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
)

// NewConfig returns a AWS session and connection Config
func NewConfig(region, roleARN string) (*session.Session, *aws.Config) {
	regionPtr := aws.String(region)
	sess := session.Must(session.NewSession(&aws.Config{Region: regionPtr, CredentialsChainVerboseErrors: aws.Bool(true)}))
	config := &aws.Config{Credentials: stscreds.NewCredentials(sess, roleARN), Region: regionPtr}
	return sess, config
}
