// Package main for the ratings simulator. Most of this was inspired by (or
// rewritten from) github.com/adrianco/spigo . Definitely the basic idea of
// having a main controller and a bunch of channel passing actors.
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/apiarian/go-ipgs/sim/tooling/gotocol"
)

func main() {
	var players int

	flag.IntVar(&players, "p", 100, "number of players")

	flag.Parse()

	log.Println("lets go")
	log.Printf("going to set up %v players\n", players)

	listener := make(chan gotocol.Message)

	controls := make(map[string]chan gotocol.Message, players)

	const gameWorldName = "game-world"

	for i := 0; i < players; i++ {
		name := fmt.Sprintf("%d-player", i)
		controls[name] = make(chan gotocol.Message)
		go RunPlayer(controls[name])
	}

	controls[gameWorldName] = make(chan gotocol.Message)
	go RunGameWorld(controls[gameWorldName])

	for name, control := range controls {
		gotocol.Message{gotocol.Hello, listener, name}.Send(control)
	}

	time.Sleep(time.Second)

	log.Println("shutting them down")
	for _, control := range controls {
		gotocol.Message{gotocol.Goodbye, nil, nil}.GoSend(control)
	}

	var msg gotocol.Message
	for len(controls) > 0 {
		msg = <-listener
		switch msg.Type {
		case gotocol.Goodbye:
			name, ok := msg.Payload.(string)
			if ok {
				log.Println("shut down thing", name)
				delete(controls, name)
			}
		}
	}
}

type PlayerInfo struct {
	Skill float64
}

func RunPlayer(listener chan gotocol.Message) {
	var name string
	var controller, gameWorld chan gotocol.Message
	var info PlayerInfo

	for {
		select {
		case msg := <-listener:
			switch msg.Type {
			case gotocol.Hello:
				n, ok := msg.Payload.(string)
				if ok && name == "" {
					controller = msg.ResponseChan
					name = n
					log.Printf("player named: %s", name)
				}

			case gotocol.JoinWorld:
				i, ok := msg.Payload.(PlayerInfo)
				if ok && gameWorld == nil && listener != nil {
					gameWorld = msg.ResponseChan
					info = i
					log.Printf("got info: %+v\n", info)
					gotocol.Message{gotocol.JoinWorld, listener, name}.GoSend(gameWorld)
				}

			case gotocol.Goodbye:
				gotocol.Message{gotocol.Goodbye, nil, name}.GoSend(controller)
				return
			}
		}
	}
}

func RunGameWorld(listener chan gotocol.Message) {
	var controller chan gotocol.Message
	var name string

	type playerInfo struct {
		info     PlayerInfo
		listener chan gotocol.Message
	}
	var players map[string]playerInfo

	for {
		select {
		case msg := <-listener:
			switch msg.Type {
			case gotocol.Hello:
				n, ok := msg.Payload.(string)
				if ok && name == "" {
					controller = msg.ResponseChan
					name = n
					log.Printf("game world named: %s", name)
				}

			case gotocol.JoinWorld:
				n, ok := msg.Payload.(string)
				if ok {
				}

			case gotocol.Goodbye:
				gotocol.Message{gotocol.Goodbye, nil, name}.GoSend(controller)
				return
			}
		}
	}
}
