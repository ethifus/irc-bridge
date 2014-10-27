package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"text/template"

	"github.com/thoj/go-ircevent"
)

type ServerConfig struct {
	Name    string
	Adress  string
	Channel string
}

type Configuration struct {
	Nicks    []string
	Servers  []ServerConfig
	Template string
}

type Message struct {
	Sender string
	Nick   string
	Body   string
}

type ServerConfigAll struct {
	Server   ServerConfig
	Nicks    []string
	Sink     chan Message
	Reciver  chan Message
	Template *template.Template
}

var logger = log.New(os.Stdout, "irc_bridge:", log.LstdFlags)

// load configuration fron JSON file into Configuration structure
func loadConfig(configPath string) *Configuration {
	configFile, err := os.Open(configPath)
	if err != nil {
		logger.Println(err.Error())
		logger.Fatal("Can't open config file.")
	}

	var config Configuration
	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&config); err != nil {
		logger.Println(err.Error())
		logger.Fatal("Can't parse config file.")
	}

	return &config
}

// Connect to single server and join desired channel.
func makeConnection(config ServerConfigAll) {
	serverName := config.Server.Name
	logger.Printf("[%s] (%s)\n", serverName, config.Server.Adress)
	nickIndex := 0
	conn := irc.IRC(config.Nicks[nickIndex], "Pan_Bot")
	//conn.VerboseCallbackHandler = true
	//conn.Debug = true

	err := conn.Connect(config.Server.Adress)
	if err != nil {
		logger.Println(err.Error())
		logger.Fatal("Can't connect to server.")
	}

	// join desired irc channel on connection success
	conn.AddCallback("001", func(e *irc.Event) {
		logger.Printf("[%s] join -> %s\n", serverName, config.Server.Channel)
		conn.Join(config.Server.Channel)
	})

	// select another nickserverName if nick is already in use
	conn.AddCallback("433", func(e *irc.Event) {
		nickIndex++
		logger.Printf("[%s] 433 trying change nick to %s\n", serverName,
			config.Nicks[nickIndex])
		conn.Nick(config.Nicks[nickIndex])
	})

	// recive message on irc channel and send it to config.Sink
	conn.AddCallback("PRIVMSG", func(e *irc.Event) {
		logger.Printf("[%s] <%s>: %s", serverName, e.Nick, e.Message())
		config.Sink <- Message{
			Sender: serverName,
			Nick:   e.Nick,
			Body:   e.Message(),
		}
	})

	go func() {
		// wait for messsages on config.Reciver and write to irc channel only
		// those that are not recived on this server/channel
		for {
			message := <-config.Reciver
			logger.Printf("[%s] recived message: %s\n", serverName, message)
			if message.Sender != serverName {
				var buffer bytes.Buffer

				err := config.Template.Execute(&buffer, message)
				if err != nil {
					logger.Fatalf("Invalid template: %s\n", err)
				}

				conn.Privmsg(config.Server.Channel, buffer.String())
			}
		}
	}()

	go conn.Loop()
}

func makeConnections(config *Configuration, sink chan Message) []chan Message {
	recivers := make([]chan Message, len(config.Servers))

	tmpl, err := template.New("message").Parse(config.Template)
	if err != nil {
		logger.Fatal("Could not crate message template: %s", err)
	}

	for i, server := range config.Servers {
		reciver := make(chan Message)
		serverConfig := ServerConfigAll{
			Server:   server,
			Nicks:    config.Nicks,
			Sink:     sink,
			Reciver:  reciver,
			Template: tmpl,
		}
		recivers[i] = reciver
		makeConnection(serverConfig)
	}
	return recivers
}

func main() {
	if len(os.Args) == 1 {
		logger.Fatal("No config file specified.")
	}

	config := loadConfig(os.Args[1])
	sink := make(chan Message)
	recivers := makeConnections(config, sink)

	// write all messages recived from sink to all recivers
	for {
		message := <-sink
		logger.Printf("recived message: [%s] <%s>: %s\n",
			message.Sender, message.Nick, message.Body)
		for i, reciver := range recivers {
			logger.Printf("sending message to reciver nr %d\n", i)
			reciver <- message
		}
	}
}
