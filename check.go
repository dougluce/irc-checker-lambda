package main

/* Horph IRC checker lambda

   Makes sure the IRC server is up and accepting connections.  Also
   make sure the designated user is online from the expected host and
   that the server has been up since the last check.

   This means it'll complain when the server reboots. But it should
   only complain one time and the server should not reboot regularly.

Environment variables:

SERVER -- the name of the server as specified in the TLS certificate.
ADDRESS -- the DNS hostname or IP address of the server
PORT -- The TCP port the server is listening to
CHECKNICK -- nickname of a user who should be online
EXPECTEDHOSTNAME -- the hostname the user should be on from
INTERVAL -- if the server uptime is less than this, complain. */

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	runtime "github.com/aws/aws-lambda-go/lambda"
	irc "github.com/thoj/go-ircevent"
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
	c.TLSConfig = &tls.Config{ServerName: os.Getenv("SERVER")}
	return c
}

func (c Connection) noNick(e *irc.Event) {
	nick := e.Arguments[1]
	anError = fmt.Errorf("Could not find %s online", nick)
	c.Quit()
}

func (c Connection) sendWhois(e *irc.Event) {
	c.Whois(os.Getenv("CHECKNICK"))
}

func (c Connection) checkWhois(e *irc.Event) {
	user_hostname := e.Arguments[3]
	if user_hostname != os.Getenv("EXPECTEDHOSTNAME") {
		anError = fmt.Errorf("%s's host is %s instead of %s", os.Getenv("CHECKNICK"), user_hostname, os.Getenv("EXPECTEDHOSTNAME"))
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
	interval, err := strconv.Atoi(os.Getenv("INTERVAL"))
	if err != nil {
		anError = fmt.Errorf("Error converting interval to int: %s", err)
		c.Quit()
		return
	}
	if secsup < interval { // We've rebooted since the last interval, notify someone!
		anError = fmt.Errorf("Server %s up for %d seconds", os.Getenv("SERVER"), secsup)
	}
	c.Quit()
}

func handleRequest(ctx context.Context, event events.CloudWatchEvent) error {
	c := NewConnection()
	c.AddCallback(RPL_WELCOME, c.sendWhois)
	c.AddCallback(RPL_WHOISUSER, c.checkWhois)
	c.AddCallback(RPL_STATSUPTIME, c.checkStats)
	c.AddCallback(ERR_NOSUCHNICK, c.noNick)
	err := c.Connect(os.Getenv("ADDRESS") + ":" + os.Getenv("PORT"))
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
