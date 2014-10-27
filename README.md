Description
===========

IRC Bridge is a simple program that connects to two or more IRC channels by
resending all messages captured on all channels to each of connected channels
(without the one that received original message).

IRC Bridge depends on [go-ircevent](http://github.com/thoj/go-ircevent) library
so you have to download it by:

    $ go get github.com/thoj/go-ircevent

You can run IRC Bridge with:

    $ irc_bridge config.json

Configuration
-------------

 * *nicks* -- List of nicknames to try (if first is used, try second, etc).
 * *username* -- Bot username.
 * *servers* -- List of servers configurations. Each entry contains *name*,
   *address* and *channel*.
 * *template* -- Template for messages which is printed on each channel if
   message from user is captured (it uses templating system from golang package
   [text/tempate](http://golang.org/pkg/text/template/)).

See [example.json](example.json).
