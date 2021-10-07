package main

/* irc.horph.com checker

   Makes sure Horph's IRC server is up and accepting connections.
   Also make sure Doug is online from newtoma and that the server has
   been up since the last check.

   This means it'll complain when the server reboots. But it should
   only complain one time and the server should not reboot regularly. */

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	irc "github.com/thoj/go-ircevent"
)

const (
	server            = "irc.horph.com"
	port              = "6697"
	check_nick        = "doug"
	expected_hostname = "ip-192-231-221-38.ec2.internal"
	INTERVAL          = 5 * 60 // Check run interval in seconds
)

const (
	RPL_WELCOME     = "001"
	RPL_WHOISUSER   = "311"
	RPL_STATSUPTIME = "242"
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

func (c Connection) sendWhois(e *irc.Event) {
	c.Whois(check_nick)
}

func (c Connection) checkWhois(e *irc.Event) {
	user_hostname := e.Arguments[3]
	if user_hostname != expected_hostname {
		fmt.Printf("ERROR: %s's host is %s instead of %s", check_nick, user_hostname, expected_hostname)
		os.Exit(1)
	}
	c.SendRawf("stats u")
}

func (c Connection) checkStats(e *irc.Event) {
	uptime := e.Arguments[1]
	re := regexp.MustCompile(`Server up (\d+) days, (\d\d):(\d\d):(\d\d)`)
	matches := re.FindStringSubmatch(uptime)
	nums := func(index int) int {
		i, err := strconv.Atoi(matches[index])
		if err != nil {
			fmt.Printf("Error converting number: %s", err)
			os.Exit(1)
		}
		return i
	}
	days, hours, minutes, seconds := nums(1), nums(2), nums(3), nums(4)
	secsup := ((days*24+hours)*60+minutes)*60 + seconds
	if secsup < INTERVAL*2 { // We've rebooted since the last interval, notify someone!
		fmt.Printf("ERROR: Server %s up for %d seconds\n", server, secsup)
		os.Exit(1)
	}
	os.Exit(0) // It's all fine!
}

func main() {
	go func() { // backup timeout
		time.Sleep(time.Minute)
		fmt.Printf("ERROR: Check for %s timed out\n", server)
		os.Exit(1)
	}()
	c := NewConnection()
	c.AddCallback(RPL_WELCOME, c.sendWhois)
	c.AddCallback(RPL_WHOISUSER, c.checkWhois)
	c.AddCallback(RPL_STATSUPTIME, c.checkStats)
	err := c.Connect(server + ":" + port)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return
	}
	go func() {
		err := <-c.ErrorChan()
		fmt.Printf("ERROR (%s): %s\n", server, err)
		os.Exit(1)
	}()
	c.Loop()
}
