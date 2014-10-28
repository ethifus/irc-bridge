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
	Address string
	Channel string
}

type Configuration struct {
	Nicks    []string
	Username string
	Servers  []ServerConfig
	Template string
}

type ServerConfigAll struct {
	Server   ServerConfig
	Nicks    []string
	Username string
	Sink     chan Message
	Reciver  chan Message
	Template *template.Template
}

type Message struct {
	Sender  string // Server's name, same as in ServerConfig.Name
	Channel string // Channel name
	Nick    string
	Body    string
}

var logger = log.New(os.Stdout, "irc_bridge:", log.LstdFlags)

// load configuration fron JSON file into Configuration structure
func loadConfig(configPath string) *Configuration {
	configFile, err := os.Open(configPath)
	if err != nil {
		logger.Println(err.Error())
		logger.Fatal("Can't open config file.")
	}
	defer configFile.Close()

	var config Configuration
	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&config); err != nil {
		logger.Println(err.Error())
		logger.Fatal("Can't parse config file.")
	}

	return &config
}

// Setup all callbacks for given connection.
func setupCallbacks(conn *irc.Connection, config ServerConfigAll) {
	serverName := config.Server.Name

	// join desired irc channel on connection success
	conn.AddCallback("001", func(e *irc.Event) {
		logger.Printf("[%s] // join -> %s\n", serverName, config.Server.Channel)
		conn.Join(config.Server.Channel)
	})

	// select another nickserverName if nick is already in use
	nickIndex := 0
	conn.AddCallback("433", func(e *irc.Event) {
		nickIndex++
		logger.Printf("[%s] // 433 trying change nick to %s\n", serverName,
			config.Nicks[nickIndex])
		conn.Nick(config.Nicks[nickIndex])
	})

	// recive message on irc channel and send it to config.Sink
	conn.AddCallback("PRIVMSG", func(e *irc.Event) {
		message := Message{
			Sender:  serverName,
			Channel: config.Server.Channel,
			Nick:    e.Nick,
			Body:    e.Message(),
		}
		logger.Println(formatMessage(config.Template, message))
		config.Sink <- message
	})
}

// Convert given message to string usign given template.
func formatMessage(template *template.Template, message Message) string {
	var buffer bytes.Buffer

	err := template.Execute(&buffer, message)
	if err != nil {
		logger.Fatalf("Invalid template: %s\n", err)
	}

	return buffer.String()
}

// Connect to single server and join desired channel.
func makeConnection(config ServerConfigAll) {
	logger.Printf("[%s] (%s/%s)\n", config.Server.Name, config.Server.Address,
		config.Server.Channel)

	conn := irc.IRC(config.Nicks[0], config.Username)
	//conn.VerboseCallbackHandler = true
	//conn.Debug = true

	err := conn.Connect(config.Server.Address)
	if err != nil {
		logger.Println(err.Error())
		logger.Fatal("Can't connect to server.")
	}

	setupCallbacks(conn, config)

	go func() {
		// wait for messsages on config.Reciver and write to irc channel only
		// those that are not recived on this server/channel
		for {
			message := <-config.Reciver
			if message.Sender != config.Server.Name {
				text := formatMessage(config.Template, message)
				conn.Privmsg(config.Server.Channel, text)
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
			Username: config.Username,
			Sink:     sink,
			Reciver:  reciver,
			Template: tmpl,
		}
		recivers[i] = reciver
		makeConnection(serverConfig)
	}
	return recivers
}

// Write all messages recived from sink to all recivers.
func loop(sink chan Message, recivers []chan Message) {
	for {
		message := <-sink
		for _, reciver := range recivers {
			reciver <- message
		}
	}
}

func main() {
	if len(os.Args) == 1 {
		logger.Fatal("No config file specified.")
	}

	config := loadConfig(os.Args[1])
	sink := make(chan Message)
	recivers := makeConnections(config, sink)

	loop(sink, recivers)
}
