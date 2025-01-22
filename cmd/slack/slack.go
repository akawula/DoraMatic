package main

import (
	"github.com/akawula/DoraMatic/slack"
	"github.com/akawula/DoraMatic/store"
)

func main() {
	prs := []store.SecurityPR{}
	slack.SendMessage(prs)
}
