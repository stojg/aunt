package core

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type RowSorter [][]string

func (l RowSorter) Len() int           { return len(l) }
func (l RowSorter) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l RowSorter) Less(i, j int) bool { return l[i][0] < l[j][0] }

// NewConfig returns a AWS session and connection Config
func NewCredentials(region, roleARN string) (*session.Session, *aws.Config) {
	regionPtr := aws.String(region)
	sess := session.Must(session.NewSession(&aws.Config{Region: regionPtr, CredentialsChainVerboseErrors: aws.Bool(true)}))
	config := &aws.Config{Credentials: stscreds.NewCredentials(sess, roleARN), Region: regionPtr}
	return sess, config
}

func TagValue(key string, tags []*ec2.Tag) string {
	for _, tag := range tags {
		if *tag.Key == key && len(*tag.Value) > 0 {
			return *tag.Value
		}
	}
	return ""
}

type keyValue struct {
	Key   string
	Value string
}

type KeyValue []keyValue

func (r *KeyValue) Add(key, value string) {
	*r = append(*r, keyValue{Key: key, Value: value})
}

func (r KeyValue) Keys() []string {
	var k []string
	for _, kv := range r {
		k = append(k, kv.Key)
	}
	return k
}

func (r KeyValue) Values() []string {
	var v []string
	for _, kv := range r {
		v = append(v, kv.Value)
	}
	return v
}
