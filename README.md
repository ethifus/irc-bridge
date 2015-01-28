Description
===========

IRC Bridge is a simple program that connects two or more IRC channels by
resending all messages captured on all channels to each of connected channels
(without the one that received original message).

Build
-----

IRC Bridge depends on [go-ircevent](http://github.com/thoj/go-ircevent) library
so you have to download it by:

    $ go get github.com/thoj/go-ircevent

You can build and run IRC Bridge with standard *go build* command:

    $ go build irc_bridge.go
    $ ./irc_bridge config.json

Configuration
-------------

Configuration is stored in JSON format. Those are the accepted fields:

 * *nicks* -- List of nicknames to try (if first is used, try second, etc).
 * *username* -- Bot username.
 * *networks* -- List of notwork configurations. Each entry contains *name*,
   *address* and *channel*.
 * *forward* -- List of names of events to catch and resend to all
   channels. Those can be any valid eventcode used by go-ircevent library
   (eg. PRIVMSG, CTCP_ACTION, NICK).
 * *templates* -- Mapping of eventcodes (from *forward*) to templates describing
   how to print given message types (it uses template system from golang
   package [text/tempate](http://golang.org/pkg/text/template/)).

See [example.json](example.json).

Tips
----

It is possible to use color codes nad font formatting in messages templates,
e.g. to show part of CTCP_ACTION message in bold use:

    "CTCP_ACTION": "{{.Network}} \u0002*{{.Nick}}\u000F {{.Message}}"
