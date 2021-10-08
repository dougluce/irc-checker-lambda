package main

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	irc "github.com/thoj/go-ircevent"
)

var cloudWatchEvent = []byte(`{
  "version": "0",
  "id": "89d1a02d-5ec7-412e-82f5-13505f849b41",
  "detail-type": "Scheduled Event",
  "source": "aws.events",
  "account": "123456789012",
  "time": "2016-12-30T18:44:49Z",
  "region": "us-east-1",
  "resources": ["arn:aws:events:us-east-1:123456789012:rule/SampleRule"],
  "detail": {}
}`)

var ctx context.Context
var event events.CloudWatchEvent

var _ = BeforeSuite(func() {
	os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "blank-go")
	d := time.Now().Add(50 * time.Millisecond)
	ctx, _ := context.WithDeadline(context.Background(), d)
	ctx = lambdacontext.NewContext(ctx, &lambdacontext.LambdaContext{
		AwsRequestID:       "495b12a8-xmpl-4eca-8168-160484189f99",
		InvokedFunctionArn: "arn:aws:lambda:us-east-2:123456789012:function:blank-go",
	})
	err := json.Unmarshal(cloudWatchEvent, &event)
	Ω(err).ShouldNot(HaveOccurred())
})

var _ = Describe("Main", func() {
	var oldAddress string
	var oldPort string
	var oldCheckNick string
	var oldExpectedHostname string

	BeforeEach(func() {
		oldAddress = address
		oldPort = port
		oldCheckNick = check_nick
		oldExpectedHostname = expected_hostname
		address = "irc.horph.com"
	})
	AfterEach(func() {
		address = oldAddress
		port = oldPort
		check_nick = oldCheckNick
		expected_hostname = oldExpectedHostname
	})
	It("works fine", func() {
		err := handleRequest(ctx, event)
		Ω(err).ShouldNot(HaveOccurred())
	})
	It("alerts when cert is bad", func() {
		address = "expired.badssl.com"
		port = "443"
		err := handleRequest(ctx, event)
		Ω(err).Should(MatchError(MatchRegexp("^x509: certificate has expired or is not yet valid:")))
	})
	It("alerts when nick is not there", func() {
		check_nick = "nobodynowhere"
		err := handleRequest(ctx, event)
		Ω(err).Should(MatchError("Could not find nobodynowhere online"))
	})
	It("alerts when nick's host is wrong", func() {
		expected_hostname = "cnn.com"
		err := handleRequest(ctx, event)
		Ω(err).Should(MatchError("doug's host is ip-192-231-221-38.ec2.internal instead of cnn.com"))
	})
	It("errors when server gives bad stats info", func() {
		event := &irc.Event{Arguments: []string{"what", "whoa"}}
		c := fakeConnection()
		c.checkStats(event)
		Ω(anError).Should(MatchError("Could not find enough info in stats call"))
	})
	It("errors when server just rebooted", func() {
		event := &irc.Event{Arguments: []string{"checker", "Server up 0 days, 00:00:10"}}
		c := fakeConnection()
		c.checkStats(event)
		Ω(anError).Should(MatchError("Server irc.horph.com up for 10 seconds"))
	})
})

// Make a Connection object that won't hang when commands are sent through it.
// Only usable for the single test that uses it above, not good enough for full testing.
func fakeConnection() Connection {
	c := NewConnection()
	rc := reflect.ValueOf(&c)
	pwrite := (*chan string)(unsafe.Pointer(rc.Elem().FieldByName("pwrite").UnsafeAddr()))
	*pwrite = make(chan string, 10)
	return c
}

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TestMain Suite")
}
