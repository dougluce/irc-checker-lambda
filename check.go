package main

/* irc.horph.com checker

   Makes sure Horph's IRC server is up and accepting connections.
   Also make sure Doug is online from newtoma and that the server has
   been up since the last check.

   This means it'll complain when the server reboots. But it should
   only complain one time and the server should not reboot regularly. */

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	runtime "github.com/aws/aws-lambda-go/lambda"
	irc "github.com/thoj/go-ircevent"
)

var (
	server            = "irc.horph.com"
	address           = "192.231.221.58"
	port              = "6697"
	check_nick        = "doug"
	expected_hostname = "ip-192-231-221-38.ec2.internal"
	INTERVAL          = 5 * 60 // Check run interval in seconds
)

var anError error

const (
	RPL_WELCOME     = "001"
	RPL_WHOISUSER   = "311"
	RPL_STATSUPTIME = "242"
	ERR_NOSUCHNICK  = "401"
)

type Connection struct {
	*irc.Connection
}

func NewConnection() Connection {
	c := Connection{irc.IRC("checker", "IRCTestSSL")}
	//c.VerboseCallbackHandler = true // Uncomment to debug
	c.Log = log.New(ioutil.Discard, "", log.LstdFlags) // to suppress connection message
	c.UseTLS = true
	c.TLSConfig = &tls.Config{ServerName: server}
	return c
}

func (c Connection) noNick(e *irc.Event) {
	nick := e.Arguments[1]
	anError = fmt.Errorf("Could not find %s online", nick)
	c.Quit()
}

func (c Connection) sendWhois(e *irc.Event) {
	c.Whois(check_nick)
}

func (c Connection) checkWhois(e *irc.Event) {
	user_hostname := e.Arguments[3]
	if user_hostname != expected_hostname {
		anError = fmt.Errorf("%s's host is %s instead of %s", check_nick, user_hostname, expected_hostname)
		c.Quit()
		return
	}
	c.SendRawf("stats u")
}

func (c Connection) checkStats(e *irc.Event) {
	uptime := e.Arguments[1]
	re := regexp.MustCompile(`Server up (\d+) days, (\d\d):(\d\d):(\d\d)`)
	matches := re.FindStringSubmatch(uptime)
	if len(matches) < 5 {
		anError = fmt.Errorf("Could not find enough info in stats call")
		c.Quit()
		return
	}
	nums := func(index int) int {
		i, err := strconv.Atoi(matches[index])
		if err != nil {
			anError = fmt.Errorf("Error converting number: %s", err)
			c.Quit()
			return -1
		}
		return i
	}
	days, hours, minutes, seconds := nums(1), nums(2), nums(3), nums(4)
	secsup := ((days*24+hours)*60+minutes)*60 + seconds
	if secsup < INTERVAL*2 { // We've rebooted since the last interval, notify someone!
		anError = fmt.Errorf("Server %s up for %d seconds", server, secsup)
	}
	c.Quit()
}

func handleRequest(ctx context.Context, event events.CloudWatchEvent) error {
	c := NewConnection()
	c.AddCallback(RPL_WELCOME, c.sendWhois)
	c.AddCallback(RPL_WHOISUSER, c.checkWhois)
	c.AddCallback(RPL_STATSUPTIME, c.checkStats)
	c.AddCallback(ERR_NOSUCHNICK, c.noNick)
	err := c.Connect(address + ":" + port)
	if err != nil {
		return err
	}
	go func() {
		err := <-c.ErrorChan()
		anError = err
		c.Quit()
	}()
	c.Loop()
	return anError
}

func main() {
	runtime.Start(handleRequest)
}
